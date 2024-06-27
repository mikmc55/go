package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/daniwalter001/jackett_fiber/types"
)

// initApp initializes the Fiber app and returns it
func initApp() *fiber.App {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("Working")
	})

	app.Get("/manifest.json", func(c *fiber.Ctx) error {
		manifest := types.StreamManifest{
			ID:          "addon_id",
			Version:     "1.0.0",
			Name:        "GoDon",
			Description: "Random Golang version on stremio Addon",
			Resources:   []string{"stream"},
			Types:       []string{"movie", "series", "anime"},
			IDPrefixes:  []string{"tt", "kitsu"},
			Catalogs:    []string{},
			Logo:        "https://upload.wikimedia.org/wikipedia/commons/2/23/Golang.png",
			ContactEmail: "your-email@example.com", // Replace with your contact email
		}

		response, err := json.Marshal(manifest)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}

		c.Set("Content-Type", "application/json")
		return c.Status(http.StatusOK).Send(response)
	})

	return app
}

// Handler is the exported function for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	adaptor.FiberApp(initApp())(w, r)
}
