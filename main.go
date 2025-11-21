package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

type LotteryResponse struct {
	RequestStatus string   `json:"requestStatus"`
	Errors        []string `json:"errors"`
	Draw          Draw     `json:"draw"`
}

type Draw struct {
	ID            int64         `json:"id"`
	Number        int           `json:"number"`
	Date          string        `json:"date"`
	SuperPrize    int64         `json:"superPrize"`
	Winners       []Winner      `json:"winners"`
	WinCategories []WinCategory `json:"winCategories"`
	TicketCount   int           `json:"ticketCount"`
	Completed     bool          `json:"completed"`
	Status        string        `json:"status"`
	Game          string        `json:"game"`
}

type Winner struct {
	Participants int `json:"participants"`
	Amount       int `json:"amount"`
	Category     int `json:"category"`
}

type WinCategory struct {
	Number       int   `json:"number"`
	Participants int   `json:"participants"`
	Amount       int   `json:"amount"`
	Numbers      []int `json:"numbers"`
}

type LotteryClient struct {
	BaseURL    string
	PartnerID  string
	HTTPClient *http.Client
}

func NewLotteryClient(partnerID string) *LotteryClient {
	return &LotteryClient{
		BaseURL:   "https://www.stoloto.ru/p/api/mobile/api/v35/service/draws",
		PartnerID: partnerID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *LotteryClient) GetDrawData(zabava string, drawID int) (*LotteryResponse, error) {
	url := fmt.Sprintf("%s/%s/%d", c.BaseURL, zabava, drawID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Gosloto-Partner", c.PartnerID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LotteryAPI/1.0)")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var lotteryResp LotteryResponse
	if err := json.Unmarshal(body, &lotteryResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return &lotteryResp, nil
}

// Обработчик для API
func drawHandler(client *LotteryClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем CORS (если нужно)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		// Получаем параметры из URL пути
		// Ожидаем URL вида /draw/ruslotto/158996
		path := r.URL.Path
		var zabava string
		var drawIDStr string

		// Парсим путь /draw/{zabava}/{drawID}
		_, err := fmt.Sscanf(path, "/draw/%s/%s", &zabava, &drawIDStr)
		if err != nil {
			fmt.Print(err)
			http.Error(w, `{"error": "Invalid URL format. Use /draw/{zabava}/{drawID}"}`, http.StatusBadRequest)
			return
		}

		// Конвертируем drawID в число
		drawID, err := strconv.Atoi(drawIDStr)
		if err != nil {
			http.Error(w, `{"error": "Draw ID must be a number"}`, http.StatusBadRequest)
			return
		}

		// Получаем данные от Stoloto API
		drawData, err := client.GetDrawData(zabava, drawID)
		if err != nil {
			errorMsg := fmt.Sprintf(`{"error": "Failed to fetch draw data: %s"}`, err.Error())
			http.Error(w, errorMsg, http.StatusInternalServerError)
			return
		}

		// Возвращаем данные клиенту
		json.NewEncoder(w).Encode(drawData)
	}
}

// Альтернативный обработчик с query parameters
func drawQueryHandler(client *LotteryClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		// Получаем параметры из query string
		// Пример: /draw?game=ruslotto&id=158996
		zabava := r.URL.Query().Get("game")
		drawIDStr := r.URL.Query().Get("id")
		// Hi

		if zabava == "" || drawIDStr == "" {
			http.Error(w, `{"error": "Missing parameters: game and id required"}`, http.StatusBadRequest)
			return
		}

		drawID, err := strconv.Atoi(drawIDStr)
		if err != nil {
			http.Error(w, `{"error": "Draw ID must be a number"}`, http.StatusBadRequest)
			return
		}

		drawData, err := client.GetDrawData(zabava, drawID)
		if err != nil {
			errorMsg := fmt.Sprintf(`{"error": "Failed to fetch draw data: %s"}`, err.Error())
			http.Error(w, errorMsg, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(drawData)
	}
}

// Health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func main() {
	// Создаем клиент для Stoloto API
	client := NewLotteryClient("bXMjXFRXZ3coWXh6R3s1NTdUX3dnWlBMLUxmdg")

	// Настраиваем роутинг
	http.HandleFunc("/draw/", drawHandler(client))
	http.HandleFunc("/draw", drawQueryHandler(client))
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Lottery API Server",
			"endpoints": `
				GET /draw/{game}/{id} - Get draw data by path parameters
				GET /draw?game={game}&id={id} - Get draw data by query parameters
				GET /health - Health check
			`,
		})
	})

	// Запускаем сервер
	port := ":8080"
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("📝 Endpoints:")
	log.Printf("   http://localhost%s/draw/ruslotto/158996", port)
	log.Printf("   http://localhost%s/draw?game=ruslotto&id=158996", port)
	log.Printf("   http://localhost%s/health", port)

	log.Fatal(http.ListenAndServe(port, nil))
}
