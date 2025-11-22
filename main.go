package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "DOUPIG/docs" // важно: замените на ваш путь

	httpSwagger "github.com/swaggo/http-swagger"
)

const (
	baseURL      = "https://www.stoloto.ru/p/api/mobile/api/v35"
	partnerToken = "bXMjXFRXZ3coWXh6R3s1NTdUX3dnWlBMLUxmdg"
	userAgent    = "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15"
)

// @title Stoloto API Proxy
// @version 1.0
// @description Прокси-сервер для API Stoloto
// @host localhost:8080
// @BasePath /

func main() {
	// Регистрируем handlers
	http.HandleFunc("/api/draws/", handleDraws)
	http.HandleFunc("/api/draw/", handleDraw)
	http.HandleFunc("/api/draw/latest", handleDrawLatest)
	http.HandleFunc("/api/draw/prelatest", handleDrawPreLatest)
	http.HandleFunc("/api/draw/momental", handleMomentalCards)

	// Swagger UI
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	log.Println("Server starting on :8080")
	log.Println("Swagger UI available at http://localhost:8080/swagger/index.html")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// GetDraws godoc
// @Summary Получить список всех игр
// @Description Возвращает информацию о всех доступных играх
// @Tags draws
// @Produce json
// @Success 200 {object} map[string]interface{} "Успешный ответ"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/draws/ [get]
func handleDraws(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := handleDrawsHandle()
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	forwardResponse(w, resp)
}

// GetDraws godoc
// @Summary Получить список всех моментальных
// @Description Возвращает информацию о всех моментальных играх
// @Tags draws
// @Produce json
// @Success 200 {object} map[string]interface{} "Успешный ответ"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/draw/momental [get]
func handleMomentalCards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := handleMomentalHandle()
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	forwardResponse(w, resp)
}

// GetDraw godoc
// @Summary Получить информацию о конкретном розыгрыше
// @Description Возвращает данные о конкретном розыгрыше по имени игры и номеру
// @Tags draws
// @Produce json
// @Param name query string true "Название игры (например: 5x36, 6x45)"
// @Param number query string true "Номер розыгрыша"
// @Success 200 {object} map[string]interface{} "Успешный ответ"
// @Failure 400 {object} ErrorResponse "Неверные параметры"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/draw/ [get]
func handleDraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем параметры из query string
	query := r.URL.Query()
	name := query.Get("name")
	number := query.Get("number")

	resp, err := handleDrawHandle(name, number)
	if err != nil {
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	forwardResponse(w, resp)
}

// GetPreLatestDraw godoc
// @Summary Получить предпоследний розыгрыш для игры
// @Description Возвращает данные последнего розыгрыша для указанной игры
// @Tags draws
// @Produce json
// @Param name query string true "Название игры (например: 5x36, 6x45)"
// @Success 200 {object} map[string]interface{} "Успешный ответ"
// @Failure 400 {object} ErrorResponse "Не указано имя игры"
// @Failure 404 {object} ErrorResponse "Игра не найдена"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/draw/prelatest [get]
func handleDrawPreLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		sendError(w, `Missing required parameter: "name"`, http.StatusBadRequest)
		return
	}

	log.Printf("Searching for latest draw of game: %s", name)

	// 1. Получаем список игр через handleDrawsHandle
	gamesResp, err := handleDrawsHandle()
	if err != nil {
		sendError(w, "Error fetching games list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer gamesResp.Body.Close()

	// 2. Читаем весь ответ
	body, err := io.ReadAll(gamesResp.Body)
	if err != nil {
		sendError(w, "Error reading games response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Парсим JSON в map
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("JSON parse error: %v", err)
		sendError(w, "Error parsing JSON response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Проверяем requestStatus поле вместо success
	requestStatus, ok := result["requestStatus"].(string)
	if !ok || requestStatus != "success" {
		log.Printf("API returned requestStatus not 'success': %v", result)
		sendError(w, "External API returned error", http.StatusInternalServerError)
		return
	}

	// 5. Извлекаем games поле
	games, ok := result["games"]
	if !ok {
		log.Printf("Missing 'games' field in response: %v", result)
		sendError(w, "Invalid API response format: missing games", http.StatusInternalServerError)
		return
	}

	// 6. Преобразуем games в массив
	gamesArray, ok := games.([]interface{})
	if !ok {
		log.Printf("Games field is not an array: %T %v", games, games)
		sendError(w, "Invalid API response format: games is not array", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d games in response", len(gamesArray))

	// 7. Ищем игру по имени
	var latestNumber int
	gameFound := false

	for i, item := range gamesArray {
		game, ok := item.(map[string]interface{})
		if !ok {
			log.Printf("Item %d is not a map: %T", i, item)
			continue
		}

		gameName, ok := game["name"].(string)
		if !ok {
			log.Printf("Game %d has no name field", i)
			continue
		}

		log.Printf("Checking game: %s", gameName)

		if gameName == name {
			gameFound = true

			// Ищем последний розыгрыш - проверяем оба поля: draw и completedDraw
			var latestDraw map[string]interface{}

			// Сначала проверяем активный розыгрыш (draw)
			if draw, exists := game["draw"]; exists && draw != nil {
				if drawMap, ok := draw.(map[string]interface{}); ok {
					latestDraw = drawMap
					log.Printf("Using active draw for %s", name)
				}
			}

			// Если нет активного, используем завершенный (completedDraw)
			if latestDraw == nil {
				if completedDraw, exists := game["completedDraw"]; exists && completedDraw != nil {
					if completedDrawMap, ok := completedDraw.(map[string]interface{}); ok {
						latestDraw = completedDrawMap
						log.Printf("Using completed draw for %s", name)
					}
				}
			}

			if latestDraw == nil {
				log.Printf("Game %s has no active or completed draws", name)
				sendError(w, fmt.Sprintf("No draws found for game '%s'", name), http.StatusNotFound)
				return
			}

			// Извлекаем number из найденного розыгрыша
			number, ok := latestDraw["number"]
			if !ok {
				log.Printf("Draw has no number field: %v", latestDraw)
				sendError(w, "Draw has no number", http.StatusInternalServerError)
				return
			}

			// Конвертируем number в int (JSON numbers are float64)
			switch n := number.(type) {
			case float64:
				latestNumber = int(n)
			case int:
				latestNumber = n
			default:
				log.Printf("Number has unexpected type: %T %v", number, number)
				sendError(w, "Invalid number format", http.StatusInternalServerError)
				return
			}

			log.Printf("Found latest draw number for %s: %d", name, latestNumber)
			break
		}
	}

	if !gameFound {
		// Собираем список доступных игр для отладки
		availableGames := make([]string, 0)
		for _, item := range gamesArray {
			if game, ok := item.(map[string]interface{}); ok {
				if gameName, ok := game["name"].(string); ok {
					availableGames = append(availableGames, gameName)
				}
			}
		}
		log.Printf("Game '%s' not found. Available games: %v", name, availableGames)
		sendError(w, fmt.Sprintf("Game '%s' not found. Available games: %v", name, availableGames), http.StatusNotFound)
		return
	}

	// 8. Получаем данные розыгрыша через handleDrawHandle
	drawResp, err := handleDrawHandle(name, fmt.Sprintf("%d", latestNumber-1))
	if err != nil {
		sendError(w, "Error fetching draw data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer drawResp.Body.Close()

	// 9. Возвращаем данные розыгрыша
	forwardResponse(w, drawResp)
}

// GetLatestDraw godoc
// @Summary Получить последний розыгрыш для игры
// @Description Возвращает данные последнего розыгрыша для указанной игры
// @Tags draws
// @Produce json
// @Param name query string true "Название игры (например: 5x36, 6x45)"
// @Success 200 {object} map[string]interface{} "Успешный ответ"
// @Failure 400 {object} ErrorResponse "Не указано имя игры"
// @Failure 404 {object} ErrorResponse "Игра не найдена"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/draw/latest [get]
func handleDrawLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		sendError(w, `Missing required parameter: "name"`, http.StatusBadRequest)
		return
	}

	log.Printf("Searching for latest draw of game: %s", name)

	// 1. Получаем список игр через handleDrawsHandle
	gamesResp, err := handleDrawsHandle()
	if err != nil {
		sendError(w, "Error fetching games list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer gamesResp.Body.Close()

	// 2. Читаем весь ответ
	body, err := io.ReadAll(gamesResp.Body)
	if err != nil {
		sendError(w, "Error reading games response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Парсим JSON в map
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("JSON parse error: %v", err)
		sendError(w, "Error parsing JSON response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Проверяем requestStatus поле вместо success
	requestStatus, ok := result["requestStatus"].(string)
	if !ok || requestStatus != "success" {
		log.Printf("API returned requestStatus not 'success': %v", result)
		sendError(w, "External API returned error", http.StatusInternalServerError)
		return
	}

	// 5. Извлекаем games поле
	games, ok := result["games"]
	if !ok {
		log.Printf("Missing 'games' field in response: %v", result)
		sendError(w, "Invalid API response format: missing games", http.StatusInternalServerError)
		return
	}

	// 6. Преобразуем games в массив
	gamesArray, ok := games.([]interface{})
	if !ok {
		log.Printf("Games field is not an array: %T %v", games, games)
		sendError(w, "Invalid API response format: games is not array", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d games in response", len(gamesArray))

	// 7. Ищем игру по имени
	var latestNumber int
	gameFound := false

	for i, item := range gamesArray {
		game, ok := item.(map[string]interface{})
		if !ok {
			log.Printf("Item %d is not a map: %T", i, item)
			continue
		}

		gameName, ok := game["name"].(string)
		if !ok {
			log.Printf("Game %d has no name field", i)
			continue
		}

		log.Printf("Checking game: %s", gameName)

		if gameName == name {
			gameFound = true

			// Ищем последний розыгрыш - проверяем оба поля: draw и completedDraw
			var latestDraw map[string]interface{}

			// Сначала проверяем активный розыгрыш (draw)
			if draw, exists := game["draw"]; exists && draw != nil {
				if drawMap, ok := draw.(map[string]interface{}); ok {
					latestDraw = drawMap
					log.Printf("Using active draw for %s", name)
				}
			}

			// Если нет активного, используем завершенный (completedDraw)
			if latestDraw == nil {
				if completedDraw, exists := game["completedDraw"]; exists && completedDraw != nil {
					if completedDrawMap, ok := completedDraw.(map[string]interface{}); ok {
						latestDraw = completedDrawMap
						log.Printf("Using completed draw for %s", name)
					}
				}
			}

			if latestDraw == nil {
				log.Printf("Game %s has no active or completed draws", name)
				sendError(w, fmt.Sprintf("No draws found for game '%s'", name), http.StatusNotFound)
				return
			}

			// Извлекаем number из найденного розыгрыша
			number, ok := latestDraw["number"]
			if !ok {
				log.Printf("Draw has no number field: %v", latestDraw)
				sendError(w, "Draw has no number", http.StatusInternalServerError)
				return
			}

			// Конвертируем number в int (JSON numbers are float64)
			switch n := number.(type) {
			case float64:
				latestNumber = int(n)
			case int:
				latestNumber = n
			default:
				log.Printf("Number has unexpected type: %T %v", number, number)
				sendError(w, "Invalid number format", http.StatusInternalServerError)
				return
			}

			log.Printf("Found latest draw number for %s: %d", name, latestNumber)
			break
		}
	}

	if !gameFound {
		// Собираем список доступных игр для отладки
		availableGames := make([]string, 0)
		for _, item := range gamesArray {
			if game, ok := item.(map[string]interface{}); ok {
				if gameName, ok := game["name"].(string); ok {
					availableGames = append(availableGames, gameName)
				}
			}
		}
		log.Printf("Game '%s' not found. Available games: %v", name, availableGames)
		sendError(w, fmt.Sprintf("Game '%s' not found. Available games: %v", name, availableGames), http.StatusNotFound)
		return
	}

	// 8. Получаем данные розыгрыша через handleDrawHandle
	drawResp, err := handleDrawHandle(name, fmt.Sprintf("%d", latestNumber))
	if err != nil {
		sendError(w, "Error fetching draw data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer drawResp.Body.Close()

	// 9. Возвращаем данные розыгрыша
	forwardResponse(w, drawResp)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

// ФУНКЦИЯ ДЛЯ API - принимает name и number
func handleDrawHandle(name, number string) (*http.Response, error) {
	if name == "" || number == "" {
		return nil, fmt.Errorf(`missing required parameters: "name" and "number"`)
	}

	url := fmt.Sprintf("/service/draws/%s/%s", name, number)
	resp, err := makeAPIRequest(url)
	if err != nil {
		return nil, fmt.Errorf("error making API request: %w", err)
	}

	return resp, nil
}

// ФУНКЦИЯ ДЛЯ API - ничего не принимает
func handleDrawsHandle() (*http.Response, error) {
	resp, err := makeAPIRequest("/service/games/info-new")
	if err != nil {
		return nil, fmt.Errorf("error making API request: %w", err)
	}

	return resp, nil
}

func handleMomentalHandle() (*http.Response, error) {
	resp, err := makeMomentalrequest("https://api.stoloto.ru/cms/api/moment-cards-section?platform=OS&user-segment=ALL")
	if err != nil {
		return nil, fmt.Errorf("error making API request: %w", err)
	}

	return resp, nil
}

func makeMomentalrequest(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	setHeaders(req)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	return resp, nil
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

func makeAPIRequest(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	setHeaders(req)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	return resp, nil
}

func setHeaders(req *http.Request) {
	req.Header.Set("Gosloto-Partner", partnerToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
}

func forwardResponse(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"success": false,
	})
}
