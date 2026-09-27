package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

var profanity = []string{"kerfuffle", "sharbert", "fornax"}

type basicJsonBody struct {
	Body string `json:"body"`
}

type JsonReturnBody struct {
	CleanedBody string `json:"cleaned_body"`
}

type JsonError struct {
	Error string `json:"error"`
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
	writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	writer.Write([]byte("Metrics Reset"))
}

func validationHandler(writer http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	chirp := basicJsonBody{}

	err := decoder.Decode(&chirp)

	if err != nil {
		respondWithJsonError(writer, 500, err)
		return
	} else if len(chirp.Body) > 140 {
		respondWithJsonError(writer, 400, errors.New("chirp is too long"))
		return
	} else {
		res := JsonReturnBody{
			CleanedBody: profanityBanishment(chirp.Body),
		}
		respondWithJSON(writer, 200, res)
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
