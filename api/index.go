package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/adaptor/v2"
	"github.com/joho/godotenv"
	"github.com/daniwalter001/jackett_fiber/types"
)

// initApp initializes the Fiber app and returns it
func initApp() *fiber.App {
	// Load environment variables
	err := godotenv.Load("./.env")
	if err != nil {
		log.Fatalln("Error loading .env file")
	}

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("Working")
	})

	app.Get("/manifest.json", func(c *fiber.Ctx) error {
		manifest := types.StreamManifest{
			ID:          "strem.go.beta",
			Description: "Random Golang version on stremio Addon",
			Name:        "GoDon",
			Resources:   []string{"stream"},
			Version:     "1.0.9",
			Types:       []string{"movie", "series", "anime"},
			Logo:        "https://upload.wikimedia.org/wikipedia/commons/2/23/Golang.png",
			IdPrefixes:  []string{"tt", "kitsu"},
			Catalogs:    []string{},
		}

		response, err := json.Marshal(manifest)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		c.Set("Content-Type", "application/json")
		return c.Status(200).Send(response)
	})

	return app
}

// Handler is the exported function for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	adaptor.FiberApp(initApp())(w, r)
}
