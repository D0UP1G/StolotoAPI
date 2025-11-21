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
    baseURL      = "https://www.stoloto.ru/p/api/mobile/api/v35"
    partnerToken = "bXMjXFRXZ3coWXh6R3s1NTdUX3dnWlBMLUxmdg"
    userAgent    = "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15"
)

// Структуры для парсинга ответа от games/info-new
type GamesResponse struct {
    Success bool          `json:"success"`
    Data    []GameData    `json:"data"`
}

type GameData struct {
    Name  string `json:"name"`
    Draws []Draw `json:"draws"`
}

type Draw struct {
    Number int `json:"number"`
}

func main() {
    // Старые endpoints (СОХРАНЕНЫ!)
    http.Handle("/api/draws/", logMiddleware(http.HandlerFunc(handleDraws)))
    http.Handle("/api/draw", logMiddleware(http.HandlerFunc(handleDraw)))
    
    // Новый endpoint для получения последнего розыгрыша
    http.Handle("/api/draw/latest", logMiddleware(http.HandlerFunc(handleDrawLatest)))
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// СТАРЫЕ ФУНКЦИИ (СОХРАНЕНЫ!)

func handleDraws(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    resp, err := makeAPIRequest("/service/games/info-new")
    if err != nil {
        sendError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    forwardResponse(w, resp)
}

func handleDraw(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    name := r.URL.Query().Get("name")
    number := r.URL.Query().Get("number")

    if name == "" || number == "" {
        sendError(w, `Missing required parameters: "name" and "number"`, http.StatusBadRequest)
        return
    }

    url := fmt.Sprintf("/service/draws/%s/%s", name, number)
    resp, err := makeAPIRequest(url)
    if err != nil {
        sendError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    forwardResponse(w, resp)
}

// НОВАЯ ФУНКЦИЯ для /api/draw/latest?name={name}
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

    // 1. Делаем запрос к games/info-new чтобы получить список игр
    gamesResp, err := makeAPIRequest("/service/games/info-new")
    if err != nil {
        sendError(w, "Error fetching games list: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer gamesResp.Body.Close()

    // 2. Парсим ответ и ищем нужную игру по name
    var gamesData GamesResponse
    body, err := io.ReadAll(gamesResp.Body)
    if err != nil {
        sendError(w, "Error reading games response: "+err.Error(), http.StatusInternalServerError)
        return
    }

    if err := json.Unmarshal(body, &gamesData); err != nil {
        sendError(w, "Error parsing games data: "+err.Error(), http.StatusInternalServerError)
        return
    }

    if !gamesData.Success {
        sendError(w, "External API returned error", http.StatusInternalServerError)
        return
    }

    // 3. Ищем игру по имени
    var targetGame *GameData
    for _, game := range gamesData.Data {
        if game.Name == name {
            targetGame = &game
            break
        }
    }

    if targetGame == nil {
        sendError(w, fmt.Sprintf("Game with name '%s' not found", name), http.StatusNotFound)
        return
    }

    if len(targetGame.Draws) == 0 {
        sendError(w, fmt.Sprintf("No draws found for game '%s'", name), http.StatusNotFound)
        return
    }

    // 4. Берем первый (последний) номер розыгрыша
    latestNumber := targetGame.Draws[0].Number

    // 5. Делаем запрос к API розыгрыша с полученным номером
    drawURL := fmt.Sprintf("/service/draws/%s/%d", name, latestNumber)
    drawResp, err := makeAPIRequest(drawURL)
    if err != nil {
        sendError(w, "Error fetching draw data: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer drawResp.Body.Close()

    // 6. Возвращаем данные розыгрыша
    forwardResponse(w, drawResp)
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ (СОХРАНЕНЫ!)

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
    // Копируем заголовки
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

// Middleware для логирования
func logMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("Started %s %s", r.Method, r.URL.String())
        
        next.ServeHTTP(w, r)
        
        log.Printf("Completed %s %s in %v", r.Method, r.URL.String(), time.Since(start))
    })
}
