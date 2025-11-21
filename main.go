package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	baseURL      = "https://www.stoloto.ru/p/api/mobile/api/v35/service/draws"
	partnerToken = "bXMjXFRXZ3coWXh6R3s1NTdUX3dnWlBMLUxmdg"
)

// Структуры для ответа от внешнего API (на основе вашего примера)
type ExternalAPIResponse struct {
	RequestStatus string   `json:"requestStatus"`
	Errors        []string `json:"errors"`
	TraceID       *string  `json:"traceId"`
	Draw          *Draw    `json:"draw"`
}

type Draw struct {
	ID                 int64         `json:"id"`
	Number             int           `json:"number"`
	Date               string        `json:"date"`
	SuperPrize         int           `json:"superPrize"`
	Jackpots           []int         `json:"jackpots"`
	WinningCombination []string      `json:"winningCombination"`
	Combination        Combination   `json:"combination"`
	WinningCategories  interface{}   `json:"winningCategories"`
	VideoLink          interface{}   `json:"videoLink"`
	Winners            interface{}   `json:"winners"`
	WinCategories      []WinCategory `json:"winCategories"`
	TicketCount        int           `json:"ticketCount"`
	BetsCount          int           `json:"betsCount"`
	SummPayed          int           `json:"summPayed"`
	FundIncreased      int           `json:"fundIncreased"`
	Game               string        `json:"game"`
	DateStopSales      string        `json:"dateStopSales"`
	DateStartSales     string        `json:"dateStartSales"`
	DateStartPayouts   string        `json:"dateStartPayouts"`
	DateStopPayouts    string        `json:"dateStopPayouts"`
	DatePublication    string        `json:"datePublication"`
	BlockIntervals     interface{}   `json:"blockIntervals"`
	SuperPrizeWon      bool          `json:"superPrizeWon"`
	SecondPrizeWon     bool          `json:"secondPrizeWon"`
	TicketPrice        int           `json:"ticketPrice"`
	Completed          bool          `json:"completed"`
	Status             string        `json:"status"`
	SuperPrizeCent     int           `json:"superPrizeCent"`
	JackpotsCent       []int         `json:"jackpotsCent"`
	SummPayedCent      int           `json:"summPayedCent"`
	HasTranslation     bool          `json:"hasTranslation"`
	PayoutInfo         PayoutInfo    `json:"payoutInfo"`
	TotalPrizeFund     float64       `json:"totalPrizeFund"`
	Categories         []Category    `json:"categories"`
}

type Combination struct {
	Serialized []string `json:"serialized"`
	Structured []int    `json:"structured"`
}

type WinCategory struct {
	Number            int         `json:"number"`
	Participants      int         `json:"participants"`
	Amount            int         `json:"amount"`
	TotalAmount       int         `json:"totalAmount"`
	AmountCents       int         `json:"amountCents"`
	TotalAmountCents  int         `json:"totalAmountCents"`
	Numbers           interface{} `json:"numbers"`
	AltPrize          interface{} `json:"altPrize"`
	Description       interface{} `json:"description"`
	Title             LocaleText  `json:"title"`
	CombinationsTitle interface{} `json:"combinationsTitle"`
	SubTitle          LocaleText  `json:"subTitle"`
	Combination       interface{} `json:"combination"`
}

type LocaleText struct {
	Ru string `json:"ru"`
	En string `json:"en"`
}

type PayoutInfo struct {
	PayoutStarted bool   `json:"payoutStarted"`
	PayoutDate    string `json:"payoutDate"`
}

type Category struct {
	Number      int `json:"number"`
	Prize       int `json:"prize"`
	PrizesCount int `json:"prizesCount"`
}

// Структура для нашего API ответа
type APIResponse struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func getDrawData(draw, id string) (*ExternalAPIResponse, error) {
	url := fmt.Sprintf("%s/%s/%s", baseURL, draw, id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Gosloto-Partner", partnerToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("неверный статус код: %d, тело: %s", resp.StatusCode, string(body))
	}

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	// Парсим JSON в нашу структуру
	var apiResp ExternalAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return &apiResp, nil
}

func drawHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		response := APIResponse{
			Status:  "error",
			Message: "Метод не поддерживается",
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(response)
		return
	}

	draw := r.URL.Query().Get("draw")
	id := r.URL.Query().Get("id")

	if draw == "" || id == "" {
		response := APIResponse{
			Status:  "error",
			Message: "Необходимы параметры: draw и id",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Получаем данные из внешнего API
	externalResp, err := getDrawData(draw, id)
	if err != nil {
		response := APIResponse{
			Status:  "error",
			Message: fmt.Sprintf("Ошибка получения данных: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Проверяем статус запроса от внешнего API
	if externalResp.RequestStatus != "success" {
		response := APIResponse{
			Status:  "error",
			Message: "Внешний API вернул ошибку",
			Error:   fmt.Sprintf("Errors: %v", externalResp.Errors),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Успешный ответ
	response := APIResponse{
		Status: "success",
		Data:   externalResp,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "stoloto-api-proxy",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(response)
}

// Эндпоинт для тестирования с примером данных
func exampleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Пример данных на основе вашего JSON
	exampleData := ExternalAPIResponse{
		RequestStatus: "success",
		Errors:        []string{},
		TraceID:       nil,
		Draw: &Draw{
			ID:                 2401592224,
			Number:             1,
			Date:               "2021-02-08T17:00:00+0300",
			SuperPrize:         2000000,
			Jackpots:           []int{2000000},
			WinningCombination: []string{"17", "19", "24", "16", "14", "21", "15", "05", "22", "07", "08", "13"},
			Combination: Combination{
				Serialized: []string{"17", "19", "24", "16", "14", "21", "15", "5", "22", "7", "8", "13"},
				Structured: []int{17, 19, 24, 16, 14, 21, 15, 5, 22, 7, 8, 13},
			},
			Game:           "zabava",
			TicketPrice:    60,
			Completed:      true,
			Status:         "COMPLETED",
			HasTranslation: true,
			SuperPrizeWon:  false,
			SecondPrizeWon: false,
			SuperPrizeCent: 200000000,
			JackpotsCent:   []int{200000000},
			SummPayedCent:  5880000,
			TotalPrizeFund: 0.0,
		},
	}

	response := APIResponse{
		Status: "success",
		Data:   exampleData,
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	// Настраиваем маршруты
	http.HandleFunc("/api/draw", drawHandler)
	http.HandleFunc("/api/draw/simple", drawSimpleHandler)
	http.HandleFunc("/api/example", exampleHandler)
	http.HandleFunc("/health", healthHandler)

	// Запускаем сервер
	port := ":8080"
	log.Printf("Сервер запущен на порту %s", port)
	log.Printf("Доступные эндпоинты:")
	log.Printf("  GET /api/draw?draw={draw}&id={id} - полная информация о розыгрыше")
	log.Printf("  GET /api/draw/simple?draw={draw}&id={id} - упрощенная информация")
	log.Printf("  GET /api/example - пример ответа с тестовыми данными")
	log.Printf("  GET /health - проверка здоровья сервера")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
