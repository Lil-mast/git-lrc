package main

import (
	"embed"
	"net/http"

	"github.com/HexmosTech/git-lrc/result"
)

//go:embed static/*
var staticFiles embed.FS

type JSONTemplateData = result.JSONTemplateData
type JSONFileData = result.JSONFileData
type JSONHunkData = result.JSONHunkData
type JSONLineData = result.JSONLineData
type JSONCommentData = result.JSONCommentData

// renderPreactHTML renders the Preact-based HTML with embedded JSON data
func renderPreactHTML(data *HTMLTemplateData) (string, error) {
	return result.RenderPreactHTML(data, staticFiles)
}

// getStaticHandler returns an HTTP handler for serving static files
func getStaticHandler() http.Handler {
	return result.GetStaticHandler(staticFiles)
}

// serveStaticFile serves a specific static file
func serveStaticFile(w http.ResponseWriter, r *http.Request, filename string) error {
	return result.ServeStaticFile(w, filename, staticFiles)
}
