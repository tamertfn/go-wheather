package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

type WeatherResponse struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
}

type OpenWeatherResponse struct {
	Weather []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
}

type WeatherHistory struct {
	ID          int64     `json:"id"`
	City        string    `json:"city"`
	Temperature float64   `json:"temperature"`
	Condition   string    `json:"condition"`
	CreatedAt   time.Time `json:"created_at"`
}

func connectToRabbitMQ() (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial("amqp://weather_user:weather123@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("RabbitMQ'ya bağlanılamadı: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Kanal oluşturulamadı: %v", err)
	}

	// Exchange tanımla
	err = ch.ExchangeDeclare(
		"weather_events", // exchange adı
		"fanout",         // exchange tipi
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Fatalf("Exchange oluşturulamadı: %v", err)
	}

	return conn, ch
}

func getCityWeather(ch *amqp.Channel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		city := vars["city"]

		apiKey := os.Getenv("OPENWEATHER_API_KEY")
		url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)

		resp, err := http.Get(url)
		if err != nil {
			respondWithError(w, http.StatusServiceUnavailable, "Weather service unavailable")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			respondWithError(w, http.StatusNotFound, "City not found")
			return
		}

		if resp.StatusCode != http.StatusOK {
			respondWithError(w, resp.StatusCode, "Weather data not available")
			return
		}

		var openWeatherResp OpenWeatherResponse
		if err := json.NewDecoder(resp.Body).Decode(&openWeatherResp); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error parsing weather data")
			return
		}

		weatherHistory := WeatherHistory{
			City:        city,
			Temperature: openWeatherResp.Main.Temp,
			Condition:   openWeatherResp.Weather[0].Main,
			CreatedAt:   time.Now(),
		}

		// RabbitMQ'ya gönder
		body, err := json.Marshal(weatherHistory)
		if err != nil {
			log.Printf("JSON marshal hatası: %v", err)
		}

		err = ch.Publish(
			"weather_events", // exchange
			"",               // routing key
			false,            // mandatory
			false,            // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})
		if err != nil {
			log.Printf("RabbitMQ'ya gönderilemedi: %v", err)
		}

		respondWithJSON(w, http.StatusOK, weatherHistory)
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	conn, ch := connectToRabbitMQ()
	defer conn.Close()
	defer ch.Close()

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/weather/cities/{city}", getCityWeather(ch)).Methods("GET", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Weather Service starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
