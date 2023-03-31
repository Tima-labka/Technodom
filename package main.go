package main

import (
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    _ "github.com/go-sql-driver/mysql"
)

type Redirect struct {
    ID          int    `json:"id"`
    ActiveLink  string `json:"active_link"`
    HistoryLink string `json:"history_link"`
}

type Cache struct {
    entries map[string]string
    count   int
    limit   int
}

func NewCache(limit int) *Cache {
    return &Cache{
        entries: make(map[string]string),
        count:   0,
        limit:   limit,
    }
}

func (c *Cache) Add(key, value string) {
    if c.count >= c.limit {
        // remove the oldest entry
        for k := range c.entries {
            delete(c.entries, k)
            break
        }
        c.count--
    }
    c.entries[key] = value
    c.count++
}

func (c *Cache) Get(key string) (string, bool) {
    value, ok := c.entries[key]
    return value, ok
}

func (c *Cache) Len() int {
    return c.count
}

func main() {
    db, err := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/database")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    router := gin.Default()

    cache := NewCache(1000)

    router.GET("/admin/redirects", func(c *gin.Context) {
        limit, _ := strconv.Atoi(c.Query("limit"))
        offset, _ := strconv.Atoi(c.Query("offset"))
        rows, err := db.Query("SELECT * FROM redirects LIMIT ? OFFSET ?", limit, offset)
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        redirects := []Redirect{}
        for rows.Next() {
            var r Redirect
            err := rows.Scan(&r.ID, &r.ActiveLink, &r.HistoryLink)
            if err != nil {
                log.Fatal(err)
            }
            redirects = append(redirects, r)
        }

        c.JSON(http.StatusOK, redirects)
    })

    router.GET("/admin/redirects/:id", func(c *gin.Context) {
        id := c.Param("id")
        row := db.QueryRow("SELECT * FROM redirects WHERE id=?", id)

        var r Redirect
        err := row.Scan(&r.ID, &r.ActiveLink, &r.HistoryLink)
        if err != nil {
            log.Fatal(err)
        }
	c.JSON(http.StatusOK, r)
})

router.POST("/admin/redirects", func(c *gin.Context) {
    var r Redirect
    if err := c.ShouldBindJSON(&r); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    result, err := db.Exec("INSERT INTO redirects (active_link, history_link) VALUES (?, ?)", r.ActiveLink, r.HistoryLink)
    if err != nil {
        log.Fatal(err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        log.Fatal(err)
    }
    r.ID = int(id)
    c.JSON(http.StatusOK, r)
})

router.PATCH("/admin/redirects/:id", func(c *gin.Context) {
    id := c.Param("id")
    row := db.QueryRow("SELECT * FROM redirects WHERE id=?", id)

    var r Redirect
    err := row.Scan(&r.ID, &r.ActiveLink, &r.HistoryLink)
    if err != nil {
        log.Fatal(err)
    }

    _, err = db.Exec("UPDATE redirects SET history_link=?, active_link=? WHERE id=?", r.ActiveLink, r.HistoryLink, r.ID)
    if err != nil {
        log.Fatal(err)
    }

    c.JSON(http.StatusOK, gin.H{"message": "Redirect updated successfully"})
})

router.DELETE("/admin/redirects/:id", func(c *gin.Context) {
    id := c.Param("id")
    _, err := db.Exec("DELETE FROM redirects WHERE id=?", id)
    if err != nil {
        log.Fatal(err)
    }
    c.JSON(http.StatusOK, gin.H{"message": "Redirect deleted successfully"})
})

router.GET("/redirects", func(c *gin.Context) {
    link := c.Query("link")
    value, ok := cache.Get(link)
    if ok {
        c.Redirect(http.StatusMovedPermanently, value)
        return
    }

    row := db.QueryRow("SELECT * FROM redirects WHERE active_link=?", link)

    var r Redirect
    err := row.Scan(&r.ID, &r.ActiveLink, &r.HistoryLink)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Redirect not found"})
        return
    }

    cache.Add(link, r.HistoryLink)
    c.Redirect(http.StatusMovedPermanently, r.ActiveLink)
})

router.Run(":8080")