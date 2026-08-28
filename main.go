// tz — időzóna-feloldás koordinátából, külső szolgáltatás nélkül.
//
// Miért van: a celestial-chart könyvtárnak a beállító-űrlaphoz kell a
// megfigyelési hely UTC-eltolása. A böngésző ezt tetszőleges földrajzi pontra
// nem tudja megmondani, csak a saját zónájára. Az upstream d3-celestial ezt
// úgy oldotta meg, hogy beégette a szerző TimeZoneDB-kulcsát — vagyis minden
// beágyazó oldal a látogatói koordinátáit küldte egy harmadik félnek.
//
// Ez a szolgáltatás ugyanazt adja, de a saját infrastruktúrán és API-kulcs
// nélkül: a poligon-adat a binárisban van, futásidőben nincs külső hívás.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ringsaturn/tzf"
	_ "time/tzdata" // az IANA adatbázis a binárisba fordítva
)

type response struct {
	TimeZone      string `json:"timeZone"`
	OffsetMinutes int    `json:"offsetMinutes"`
	Abbreviation  string `json:"abbreviation"`
	Timestamp     int64  `json:"timestamp"`
}

type errorResponse struct {
	Error string `json:"error"`
}

var finder tzf.F

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// A CORS-t környezeti változó szabja meg. Alapból csak a saját oldalak
// hívhatják; "*"-gal bárki. Hitelesítés nincs, sütit nem olvasunk, ezért a
// "*" is védhető — de legyen tudatos döntés, ne alapértelmezés.
func cors(w http.ResponseWriter, r *http.Request, allowed []string) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			w.Header().Set("Access-Control-Allow-Origin", a)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "86400")
			return
		}
	}
}

func handleTimezone(allowed []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w, r, allowed)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"only GET"})
			return
		}

		q := r.URL.Query()
		lat, errLat := strconv.ParseFloat(q.Get("lat"), 64)
		lon, errLon := strconv.ParseFloat(q.Get("lon"), 64)
		if errLat != nil || errLon != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{"lat and lon must be numbers"})
			return
		}
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			writeJSON(w, http.StatusBadRequest, errorResponse{"lat must be within ±90, lon within ±180"})
			return
		}

		// Az eltolás időpontfüggő: a nyári időszámítás miatt ugyanaz a hely
		// más eltolást ad januárban és júliusban. Megadható; alapból a mostani.
		when := time.Now()
		if raw := q.Get("t"); raw != "" {
			seconds, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{"t must be a unix timestamp in seconds"})
				return
			}
			when = time.Unix(seconds, 0)
		}

		name := finder.GetTimezoneName(lon, lat) // figyelem: (lon, lat) a sorrend
		if name == "" {
			// A tzf az óceánokat is lefedi a hajózási Etc/GMT±N zónákkal, tehát
			// ez a gyakorlatban nem fordul elő — de ha mégis üresen jönne
			// vissza, inkább 404-et adunk, mint hamis eltolást.
			writeJSON(w, http.StatusNotFound, errorResponse{"no time zone at this position"})
			return
		}

		loc, err := time.LoadLocation(name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"unknown zone: " + name})
			return
		}
		abbr, offsetSeconds := when.In(loc).Zone()
		writeJSON(w, http.StatusOK, response{
			TimeZone:      name,
			OffsetMinutes: offsetSeconds / 60,
			Abbreviation:  abbr,
			Timestamp:     when.Unix(),
		})
	}
}

func main() {
	allowed := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	for i := range allowed {
		allowed[i] = strings.TrimSpace(allowed[i])
	}
	if len(allowed) == 1 && allowed[0] == "" {
		allowed = []string{"https://celestial.blackit.hu", "https://csillag.blackit.hu"}
	}

	var err error
	finder, err = tzf.NewDefaultFinder()
	if err != nil {
		log.Fatalf("could not load the time zone polygons: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/timezone", handleTimezone(allowed))
	mux.HandleFunc("/timezone/health", func(w http.ResponseWriter, r *http.Request) {
		// Egy ismert pont: ha ez üres, a poligon-adat nem töltődött be.
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"probe":  finder.GetTimezoneName(19.04, 47.50), // Budapest
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("tz listening on :%s, allowed origins: %v", port, allowed)
	log.Fatal(server.ListenAndServe())
}
