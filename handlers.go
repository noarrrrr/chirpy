package main

import (
	"database/sql"
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

const JWTsecondsForExpiration int = 3600

const RefreshTokenDaysForExpiration int = 60

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
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	JWToken      string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

type Token struct {
	Token string `json:"token"`
}

type IncomingChirp struct {
	Body string `json:"body"`
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
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithJsonError(writer, 401, err)
		return
	}

	decoder := json.NewDecoder(req.Body)
	chirp := IncomingChirp{}

	err = decoder.Decode(&chirp)

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
			UserID:    userID,
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

	jwtoken, err := auth.MakeJWT(dbUserInfo.ID, cfg.secret, time.Duration(JWTsecondsForExpiration)*time.Second)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	refreshTokenString := auth.MakeRefreshToken()

	refreshTokenParams := database.CreateRefreshTokenParams{
		Token:     refreshTokenString,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(RefreshTokenDaysForExpiration*24) * time.Hour),
		RevokedAt: sql.NullTime{Time: time.Time{}, Valid: false},
		UserID:    dbUserInfo.ID,
	}

	refreshToken, err := cfg.dbQueries.CreateRefreshToken(req.Context(), refreshTokenParams)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	dbUserWithoutPassword := User{
		ID:           dbUserInfo.ID,
		CreatedAt:    dbUserInfo.CreatedAt,
		UpdatedAt:    dbUserInfo.UpdatedAt,
		Email:        dbUserInfo.Email,
		JWToken:      jwtoken,
		RefreshToken: refreshToken.Token,
	}

	respondWithJSON(writer, 200, dbUserWithoutPassword)
}

func (cfg *apiConfig) refreshUserHandler(writer http.ResponseWriter, req *http.Request) {
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Missing Refresh Token"))
		return
	}

	refreshToken, err := cfg.dbQueries.MatchRefreshToken(req.Context(), tokenString)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Refresh Token Doesn't Exist"))
		return
	} else if time.Until(refreshToken.ExpiresAt) <= time.Microsecond {
		respondWithJsonError(writer, 401, errors.New("Refresh Token Has Expired"))
		return
	} else if refreshToken.RevokedAt.Valid == true {
		respondWithJsonError(writer, 401, errors.New("Refresh Token Has Been Revoked"))
		return
	}

	jwtoken, err := auth.MakeJWT(refreshToken.UserID, cfg.secret, time.Duration(JWTsecondsForExpiration)*time.Second)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	response := Token{
		Token: jwtoken,
	}
	respondWithJSON(writer, 200, response)
}

func (cfg *apiConfig) revokeRefreshTokenHandler(writer http.ResponseWriter, req *http.Request) {
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Missing Refresh Token"))
		return
	}

	refreshToken, err := cfg.dbQueries.MatchRefreshToken(req.Context(), tokenString)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Refresh Token Doesn't Exist"))
		return
	}

	params := database.RevokeRefreshTokenParams{
		RevokedAt: sql.NullTime{Time: time.Now(), Valid: true},
		UpdatedAt: time.Now(),
		Token:     refreshToken.Token,
	}

	err = cfg.dbQueries.RevokeRefreshToken(req.Context(), params)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	} else {
		respondWithJSON(writer, 204, basicJsonBody{})
	}
}

func (cfg *apiConfig) updateUserHandler(writer http.ResponseWriter, req *http.Request) {
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Missing JWToken"))
		return
	}

	userID, err := auth.ValidateJWT(tokenString, cfg.secret)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Invalid JWToken"))
		return
	}

	decoder := json.NewDecoder(req.Body)
	userInfo := basicJsonUser{}

	err = decoder.Decode(&userInfo)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	hashedPass, err := auth.HashPassword(userInfo.Password)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	params := database.UpdateUserParams{
		Email:          userInfo.Email,
		HashedPassword: hashedPass,
		UpdatedAt:      time.Now(),
		ID:             userID,
	}

	dbUser, err := cfg.dbQueries.UpdateUser(req.Context(), params)

	response := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(writer, 200, response)
}

func (cfg *apiConfig) deleteChirpHandler(writer http.ResponseWriter, req *http.Request) {
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Missing JWToken"))
		return
	}

	userID, err := auth.ValidateJWT(tokenString, cfg.secret)
	if err != nil {
		respondWithJsonError(writer, 401, errors.New("Invalid JWToken"))
		return
	}

	rawChirpID := req.PathValue("chirpID")
	chirpID, err := uuid.Parse(rawChirpID)
	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	}

	dbChirp, err := cfg.dbQueries.GetChirp(req.Context(), chirpID)
	if err != nil {
		respondWithJsonError(writer, 404, errors.New("Chirp not Found"))
		return
	}

	if dbChirp.UserID != userID {
		respondWithJsonError(writer, 403, errors.New("You can only delete chirps that you chirped"))
		return
	}

	err = cfg.dbQueries.DeleteChirp(req.Context(), dbChirp.ID)
	if err != nil {
		respondWithJsonError(writer, 404, errors.New("Chirp not found"))
		return
	}

	respondWithJSON(writer, 204, basicJsonBody{})
}
