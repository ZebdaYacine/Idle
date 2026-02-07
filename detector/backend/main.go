package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// DB
	conn := OpenRqliteFromEnv()
	repo := NewActivityRepo(conn)

	// HTTP
	handler := NewActivityHandler(repo)

	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
	app.Get("/activity/today", handler.GetToday)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":" + port))
}
