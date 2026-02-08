package main

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// album represents data about a record album
type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

// AlbumStore provides a thread-safe store for albums
type AlbumStore struct {
	sync.RWMutex
	albums []album
}

// Seed data
var store = AlbumStore{
	albums: []album{
		{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
		{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
		{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
	},
}

func main() {
	// Gin release mode reduces logging overhead
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.GET("/albums", getAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.POST("/albums", postAlbums)

	router.Run("0.0.0.0:8080")
}

// getAlbums responds with the list of all albums as JSON
func getAlbums(c *gin.Context) {
	// Read Lock
	store.RLock()
	defer store.RUnlock()

	c.IndentedJSON(http.StatusOK, store.albums)
}

// postAlbums adds an album from JSON received in the request body
func postAlbums(c *gin.Context) {
	var newAlbum album

	// Call BindJSON to bind the received JSON to newAlbum
	if err := c.BindJSON(&newAlbum); err != nil {
		return
	}

	// Write Lock
	store.Lock()
	store.albums = append(store.albums, newAlbum)
	store.Unlock()

	c.IndentedJSON(http.StatusCreated, newAlbum)
}

// getAlbumByID locates the album whose ID value matches the id
// parameter sent by the client, then returns that album as a response
func getAlbumByID(c *gin.Context) {
	id := c.Param("id")

	// Read Lock
	store.RLock()
	defer store.RUnlock()

	// Loop through the list of albums, looking for
	// an album whose ID value matches the parameter
	for _, a := range store.albums {
		if a.ID == id {
			c.IndentedJSON(http.StatusOK, a)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
}
