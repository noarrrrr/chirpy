package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/noarrrrr/chirpy/internal/auth"
	"github.com/noarrrrr/chirpy/internal/database"
)

var profanity = []string{"kerfuffle", "sharbert", "fornax"}

type basicJsonBody struct {
	Body string `json:"body"`
}

type basicJsonUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type JsonReturnBody struct {
	CleanedBody string `json:"cleaned_body"`
}

type JsonError struct {
	Error string `json:"error"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type IncomingChirp struct {
	UserID uuid.UUID `json:"user_id"`
	Body   string    `json:"body"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func respondWithJsonError(w http.ResponseWriter, code int, err error) {
	msg := fmt.Sprintf("%v", err)
	errStruct := JsonError{
		Error: msg,
	}
	data, err := json.Marshal(errStruct)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func respondWithJSON(w http.ResponseWriter, code int, bodyStruct any) {
	data, err := json.Marshal(bodyStruct)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		respondWithJsonError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func healthzHandler(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(200)
	writer.Write([]byte("OK"))
}

func (cfg *apiConfig) metricsHandler(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Add("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(200)
	writer.Write([]byte(fmt.Sprintf(`<html><body>
	<h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body</html>`,
		cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) incrementHits(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		handler.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) resetHandler(writer http.ResponseWriter, req *http.Request) {
	if cfg.platform != "dev" {
		respondWithJsonError(writer, 403, errors.New("Forbidden"))
	}
	err := cfg.dbQueries.ResetUsers(req.Context())
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}
	writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	writer.Write([]byte("Metrics Reset"))
}

func (cfg *apiConfig) chirpHandler(writer http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	chirp := IncomingChirp{}

	err := decoder.Decode(&chirp)

	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	} else if len(chirp.Body) > 140 {
		respondWithJsonError(writer, 400, errors.New("chirp is too long"))
		return
	} else {
		cleanBody := profanityBanishment(chirp.Body)
		params := database.CreateChirpParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Body:      cleanBody,
			UserID:    chirp.UserID,
		}
		dbChirp, err := cfg.dbQueries.CreateChirp(req.Context(), params)
		if err != nil {
			respondWithJsonError(writer, 500, err)
			return
		}
		jsonChirp := dbChirpToJson(dbChirp)
		respondWithJSON(writer, 201, jsonChirp)
	}

}

func dbChirpToJson(dbChirp database.Chirp) Chirp {
	return Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
}

func profanityBanishment(thePossiblyProfane string) string {
	splittedText := strings.Split(thePossiblyProfane, " ")
	lowercaseReferance := strings.Split(strings.ToLower(thePossiblyProfane), " ")
	for i, word := range lowercaseReferance {
		for _, badWord := range profanity {
			if word == badWord {
				splittedText[i] = "****"
			}
		}
	}
	return strings.Join(splittedText, " ")
}

func (cfg *apiConfig) chirpsRetrieveHandler(writer http.ResponseWriter, req *http.Request) {
	chirps, err := cfg.dbQueries.GetChirps(req.Context())
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	} else if len(chirps) == 0 {
		respondWithJsonError(writer, 500, errors.New("No chirps in database"))
		return
	} else {
		jsonChirps := make([]Chirp, len(chirps))
		for i, dbChirp := range chirps {
			jsonChirps[i] = dbChirpToJson(dbChirp)
		}
		respondWithJSON(writer, 200, jsonChirps)
	}

}

func (cfg *apiConfig) chirpRetrieveHandler(writer http.ResponseWriter, req *http.Request) {
	rawChirpID := req.PathValue("chirpID")
	chirpID, err := uuid.Parse(rawChirpID)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	dbChirp, err := cfg.dbQueries.GetChirp(req.Context(), chirpID)
	if err != nil {
		respondWithJsonError(writer, 404, errors.New("Not Found"))
		return
	}
	respondWithJSON(writer, 200, dbChirpToJson(dbChirp))
}

func (cfg *apiConfig) createUserHandler(writer http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	userInfo := basicJsonUser{}

	err := decoder.Decode(&userInfo)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	hashedPass, err := auth.HashPassword(userInfo.Password)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	ctx := req.Context()

	userparams := database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Email:          userInfo.Email,
		HashedPassword: hashedPass,
	}

	dbUser, err := cfg.dbQueries.CreateUser(ctx, userparams)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(writer, 201, user)
}

func (cfg *apiConfig) loginUserHandler(writer http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	userInfo := basicJsonUser{}

	err := decoder.Decode(&userInfo)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	dbUserInfo, err := cfg.dbQueries.GetUser(req.Context(), userInfo.Email)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Incorrect email or password"))
		return
	}

	sentPassword := userInfo.Password
	dbPassword := dbUserInfo.HashedPassword
	match, err := auth.CheckPasswordHash(sentPassword, dbPassword)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	} else if match == false {
		respondWithJsonError(writer, 401, errors.New("Incorrect email or password"))
		return
	}
	dbUserWithoutPassword := User{
		ID:        dbUserInfo.ID,
		CreatedAt: dbUserInfo.CreatedAt,
		UpdatedAt: dbUserInfo.UpdatedAt,
		Email:     dbUserInfo.Email,
	}
	respondWithJSON(writer, 200, dbUserWithoutPassword)
}
