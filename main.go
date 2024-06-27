package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/anacrolix/torrent"
	"github.com/daniwalter001/jackett_fiber/types"
	"github.com/daniwalter001/jackett_fiber/types/rd"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

// initApp initializes the Fiber app and returns it
func initApp() *fiber.App {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	ctx := context.Background()

	err := godotenv.Load("./.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Create Redis client instance
	rdClient := RedisClient()
	status, err := rdClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Error connecting to Redis: %v", err)
	} else {
		log.Printf("Connected to Redis: %s", status)
	}

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
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
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		c.Set("Content-Type", "application/json")
		return c.Status(fiber.StatusOK).Send(response)
	})

	app.Get("/stream/:type/:id.json", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "*")
		c.Set("Content-Type", "application/json")

		id := c.Params("id")
		id = strings.ReplaceAll(id, "%3A", ":")

		streams, err := rdClient.JSONGet(ctx, id, "$").Result()
		if err == nil && streams != "" {
			var cachedStreams []types.StreamMeta
			err = json.Unmarshal([]byte(streams), &cachedStreams)
			if err == nil && len(cachedStreams) > 0 {
				return c.Status(fiber.StatusOK).JSON(cachedStreams[len(cachedStreams)-1])
			}
		}

		type_ := c.Params("type")
		var tmp []string

		if strings.Contains(id, "kitsu") {
			tmp = getImdbFromKitsu(id)
		} else {
			tmp = strings.Split(id, ":")
		}

		var s, e, absSeason, absEpisode int
		var abs string

		tt := tmp[0]
		if len(tmp) > 1 {
			s, _ = strconv.Atoi(tmp[1])
			e, _ = strconv.Atoi(tmp[2])
			if len(tmp) > 3 {
				absSeason, _ = strconv.Atoi(tmp[3])
				absEpisode, _ = strconv.Atoi(tmp[4])
				abs = tmp[5]
			}
		}

		name, year := getMeta(tt, type_)

		var results []types.ItemsParsed
		var wg sync.WaitGroup

		switch type_ {
		case "movie":
			wg.Add(1)
			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s %s", name, year), type_)...)
			}()
		case "series":
			l := 5
			if abs == "true" {
				l += 2
			}
			if s == 1 {
				l += 2
			}

			wg.Add(l)
			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s S%02d", name, s), type_)...)
			}()
			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s integrale", name), type_)...)
			}()
			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s batch", name), type_)...)
			}()
			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s complet", name), type_)...)
			}()

			go func() {
				defer wg.Done()
				results = append(results, fetchTorrent(fmt.Sprintf("%s S%02dE%02d", name, s, e), type_)...)
			}()

			if s == 1 {
				go func() {
					defer wg.Done()
					results = append(results, fetchTorrent(fmt.Sprintf("%s E%02d", name, e), type_)...)
				}()
				go func() {
					defer wg.Done()
					results = append(results, fetchTorrent(fmt.Sprintf("%s %02d", name, e), type_)...)
				}()
			}

			if abs == "true" {
				go func() {
					defer wg.Done()
					results = append(results, fetchTorrent(fmt.Sprintf("%s E%03d", name, absEpisode), type_)...)
				}()

				go func() {
					defer wg.Done()
					results = append(results, fetchTorrent(fmt.Sprintf("%s %03d", name, absEpisode), type_)...)
				}()
			}
		}

		wg.Wait()

		results = removeDuplicates(results)
		sort.Slice(results, func(i, j int) bool {
			iv, _ := strconv.Atoi(results[i].Peers)
			jv, _ := strconv.Atoi(results[j].Peers)
			return iv > jv
		})

		maxRes, _ := strconv.Atoi(os.Getenv("MAX_RES"))
		if len(results) > maxRes {
			results = results[:maxRes]
		}

		var parsedTorrentFiles []types.ItemsParsed
		var parsedSuitableTorrentFiles []torrent.File
		parsedSuitableTorrentFilesIndex := make(map[string]int)

		for _, item := range results {
			parsedSuitableTorrentFiles = []torrent.File{}
			for idx, file := range item.TorrentData {
				if !isVideo(file.DisplayPath()) {
					continue
				}

				if type_ == "movie" {
					parsedSuitableTorrentFiles = append(parsedSuitableTorrentFiles, file)
					parsedSuitableTorrentFilesIndex[file.DisplayPath()] = idx + 1
					break
				}

				lower := strings.ToLower(file.DisplayPath())

				if containEandS(lower, strconv.Itoa(s), strconv.Itoa(e), abs == "true", strconv.Itoa(absSeason), strconv.Itoa(absEpisode)) ||
					containE_S(lower, strconv.Itoa(s), strconv.Itoa(e), abs == "true", strconv.Itoa(absSeason), strconv.Itoa(absEpisode)) ||
					(s == 1 && (containsAbsoluteE(lower, strconv.Itoa(s), strconv.Itoa(e), true, strconv.Itoa(s), strconv.Itoa(e)) ||
						containsAbsoluteE_(lower, strconv.Itoa(s), strconv.Itoa(e), true, strconv.Itoa(s), strconv.Itoa(e)))) ||
					((abs == "true" && containsAbsoluteE(lower, strconv.Itoa(s), strconv.Itoa(e), true, strconv.Itoa(absSeason), strconv.Itoa(absEpisode))) ||
						(abs == "true" && containsAbsoluteE_(lower, strconv.Itoa(s), strconv.Itoa(e), true, strconv.Itoa(absSeason), strconv.Itoa(absEpisode)))) {
					parsedSuitableTorrentFiles = append(parsedSuitableTorrentFiles, file)
					parsedSuitableTorrentFilesIndex[file.DisplayPath()] = idx + 1
					break
				}
			}
			item.TorrentData = parsedSuitableTorrentFiles
			parsedTorrentFiles = append(parsedTorrentFiles, item)
		}

		var streams []types.StreamMeta
		for _, item := range parsedTorrentFiles {
			for _, file := range item.TorrentData {
				torrent := types.StreamMeta{
					Title:         item.Title,
					InfoHash:      item.InfoHash,
					FileIdx:       parsedSuitableTorrentFilesIndex[file.DisplayPath()],
					BehaviorHints: types.BehaviorHints{BingeGroup: fmt.Sprintf("group-%s", tt), CountryWhitelist: []string{"en"}},
				}
				streams = append(streams, torrent)
			}
		}

		err = rdClient.JSONSet(ctx, id, "$", streams).Err()
		if err != nil {
			log.Printf("Error updating cache: %v", err)
		}

		return c.Status(fiber.StatusOK).JSON(streams[len(streams)-1])
	})

	return app
}

// Handler is the exported function for Vercel
var Handler = adaptor.FiberApp(initApp())

func main() {
	Handler.Listen(":3000")
}
