package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	log.Println("thumbnail content type:", contentType)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Requested video not found", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized request", errors.New("Unauthorized Request"))
		return
	}

	filePath := ""
	fileExtension := ""
	thumbnailName := ""

	contentTypeParsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot parse header (content type)", err)
		return
	}

	switch contentTypeParsed {
	case "image/png":
		fileExtension = ".png"
		byteName := make([]byte, 32)
		_, err = rand.Read(byteName)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Error naming image file", err)
		}
		thumbnailName = base64.RawURLEncoding.EncodeToString(byteName) + fileExtension
		filePath = filepath.Join(cfg.assetsRoot, thumbnailName)
	case "image/jpeg":
		fileExtension = ".jpg"
		byteName := make([]byte, 32)
		_, err = rand.Read(byteName)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Error naming image file", err)
		}
		thumbnailName = base64.RawURLEncoding.EncodeToString(byteName) + fileExtension
		filePath = filepath.Join(cfg.assetsRoot, thumbnailName)
	default:
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail format", errors.New("Invalid thumbnail format"))
		return
	}

	thumbnailFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating thumbnail file", err)
		return
	}

	defer thumbnailFile.Close()

	_, err = io.Copy(thumbnailFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying thumbnail file", err)
		return
	}

	thumbnailURLString := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, thumbnailName)

	err = cfg.db.UpdateVideoThumbnailURL(videoID, thumbnailURLString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update thumbnail", err)
		return
	}
	video.ThumbnailURL = &thumbnailURLString
	respondWithJSON(w, http.StatusOK, video)
}
