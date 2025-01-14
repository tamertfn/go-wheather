package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

type WeatherHistory struct {
	ID          int64     `json:"id"`
	City        string    `json:"city"`
	Temperature float64   `json:"temperature"`
	Condition   string    `json:"condition"`
	CreatedAt   time.Time `json:"created_at"`
}

type HistoryService struct {
	db *pgxpool.Pool
}

func NewHistoryService(db *pgxpool.Pool) *HistoryService {
	return &HistoryService{db: db}
}

func (s *HistoryService) createHistory(w http.ResponseWriter, r *http.Request) {
	var history WeatherHistory
	if err := json.NewDecoder(r.Body).Decode(&history); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	query := `
		INSERT INTO weather_history (city, temperature, condition)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := s.db.QueryRow(context.Background(), query,
		history.City, history.Temperature, history.Condition).
		Scan(&history.ID, &history.CreatedAt)

	if err != nil {
		log.Printf("Error creating history: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error creating history record")
		return
	}

	respondWithJSON(w, http.StatusCreated, history)
}

func (s *HistoryService) getCityHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	city := vars["city"]

	rows, err := s.db.Query(context.Background(),
		`SELECT id, city, temperature, condition, created_at 
		 FROM weather_history 
		 WHERE city = $1 
		 ORDER BY created_at DESC 
		 LIMIT 10`, city)

	if err != nil {
		log.Printf("Error querying history: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error retrieving history")
		return
	}
	defer rows.Close()

	var history []WeatherHistory
	for rows.Next() {
		var h WeatherHistory
		if err := rows.Scan(&h.ID, &h.City, &h.Temperature, &h.Condition, &h.CreatedAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		history = append(history, h)
	}

	respondWithJSON(w, http.StatusOK, history)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func startWeatherConsumer(db *pgxpool.Pool) {
	conn, err := amqp.Dial("amqp://weather_user:weather123@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("RabbitMQ'ya bağlanılamadı: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Kanal oluşturulamadı: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"weather_history", // queue adı
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		log.Fatalf("Queue oluşturulamadı: %v", err)
	}

	err = ch.QueueBind(
		q.Name,           // queue name
		"",               // routing key
		"weather_events", // exchange
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Queue bind edilemedi: %v", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalf("Consumer oluşturulamadı: %v", err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var history WeatherHistory
			if err := json.Unmarshal(d.Body, &history); err != nil {
				log.Printf("JSON parse hatası: %v", err)
				continue
			}

			history.CreatedAt = time.Now()

			query := `
				INSERT INTO weather_history (city, temperature, condition, created_at) 
				VALUES ($1, $2, $3, $4)
				RETURNING id`

			err := db.QueryRow(context.Background(), query,
				history.City,
				history.Temperature,
				history.Condition,
				history.CreatedAt).Scan(&history.ID)

			if err != nil {
				log.Printf("Veritabanı kayıt hatası: %v", err)
			} else {
				log.Printf("Yeni hava durumu kaydı oluşturuldu: %s, ID: %d", history.City, history.ID)
			}
		}
	}()

	log.Printf("Weather history consumer başlatıldı")
	<-forever
}

func main() {
	// PostgreSQL bağlantısı
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://weather_user:weather123@postgres:5432/weather_history"
	}

	db, err := pgxpool.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer db.Close()

	// RabbitMQ consumer'ı ayrı bir goroutine'de başlat
	go startWeatherConsumer(db)

	historyService := NewHistoryService(db)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()

	// Routes
	api.HandleFunc("/history", historyService.createHistory).Methods("POST", "OPTIONS")
	api.HandleFunc("/history/cities/{city}", historyService.getCityHistory).Methods("GET", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("History Service starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
