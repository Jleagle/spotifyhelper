package main

import (
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jleagle/spotifyhelper/logging"
	"github.com/Jleagle/spotifyhelper/session"
	"github.com/Jleagle/spotifyhelper/structs"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi"
	roll "github.com/stvp/rollbar"
	spot "github.com/zmb3/spotify"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {

	roll.Token = os.Getenv("SPOTIFY_ROLLBAR_PRIVATE")
	roll.Environment = os.Getenv("ENV")
	defer roll.Wait()

	r := chi.NewRouter()
	r.Get("/", homeHandler)
	r.Get("/album/{album}", albumHandler)
	r.Get("/artist/{artist}", artistHandler)
	r.Get("/duplicates", duplicatesHandler)
	r.Get("/duplicates/{playlist}/{new}", duplicatesActionHandler)
	r.Get("/login", loginHandler)
	r.Get("/login-callback", loginCallbackHandler)
	r.Get("/logout", logoutHandler)
	r.Get("/random", randomHandler)
	r.Get("/random/{type}", randomHandler)
	r.Get("/sitemap.xml", siteMapHandler)
	r.Get("/shuffle", shuffleHandler)
	r.Get("/shuffle/{playlist}/{new}", shuffleActionHandler)
	r.Get("/top", topHandler)
	r.Get("/top/{type}", topHandler)
	r.Get("/top/{type}/{range}", topHandler)
	r.Get("/track/{track}", trackHandler)
	r.Get("/unpopular", unpopularHandler)
	r.Get("/user/{user}", userHandler)
	r.Get("/user/{user}/playlist/{playlist}", playlistHandler)

	// Assets
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "assets")
	fileServer(r, "/assets", http.Dir(filesDir))

	log.Fatal(http.ListenAndServe(":8080", r))
}

func returnTemplate(w http.ResponseWriter, r *http.Request, page string, pageData structs.TemplateVars, err error) {

	// Log errors
	if err != nil {
		logging.Error(err)
	}

	// Check if logged in
	pageData.LoggedIn, err = session.IsLoggedIn(r)
	if err != nil {
		logging.Error(err)
	}

	pageData.Flashes, err = session.GetFlashes(w, r)
	if err != nil {
		logging.Error(err)
	}

	pageData.LoggedInID, err = session.Read(r, session.UserID)
	if err != nil {
		logging.Error(err)
	}

	if page == "error" && err != nil {
		pageData.ErrorMessage = err.Error()
	}

	pageData.Highlight = r.URL.Query().Get("highlight")
	pageData.Enviroment = os.Getenv("ENV")

	// Load templates needed
	folder := os.Getenv("SPOTIFY_PATH")
	if folder == "" {
		folder = "/root"
	}

	always := []string{
		folder + "/templates/header.html",
		folder + "/templates/footer.html",
		folder + "/templates/" + page + ".html",
		folder + "/templates/includes/album.html",
	}

	t, err := template.New("t").Funcs(getTemplateFuncMap()).ParseFiles(always...)
	if err != nil {
		logging.Critical(err)
	}

	// Write a respone
	err = t.ExecuteTemplate(w, page, pageData)
	if err != nil {
		logging.Critical(err)
	}
}

func returnLoggedOutTemplate(w http.ResponseWriter, r *http.Request, err error) {

	vars := structs.TemplateVars{}
	vars.ErrorMessage = "Please login"

	returnTemplate(w, r, "error", vars, err)
	return
}

func getTemplateFuncMap() map[string]interface{} {
	return template.FuncMap{
		"join":    func(a []string) string { return strings.Join(a, ", ") },
		"title":   func(a string) string { return strings.Title(a) },
		"comma":   func(a uint) string { return humanize.Comma(int64(a)) },
		"plusone": func(a int) int { return a + 1 },
		"bool": func(a bool) string {
			if a == true {
				return "Yes"
			} else {
				return "No"
			}
		},
		"artists": func(a []spot.SimpleArtist) template.HTML {
			var artists []string
			for _, v := range a {
				artists = append(artists, "<a href=\"/artist/"+string(v.ID)+"\">"+v.Name+"</a>")
			}
			return template.HTML(strings.Join(artists, ", "))
		},
		"genres": func(a []string) string {
			var genres []string
			for _, v := range a {
				genres = append(genres, strings.Title(v))
			}
			return strings.Join(genres, ", ")
		},
		"seconds": func(inSeconds int) string {
			inSeconds = inSeconds / 1000
			minutes := inSeconds / 60
			seconds := inSeconds % 60
			return fmt.Sprintf("%vm %vs", minutes, seconds)
		},
	}
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func fileServer(r chi.Router, path string, root http.FileSystem) {

	if strings.ContainsAny(path, "{}*") {
		//logger.ErrExit("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}
