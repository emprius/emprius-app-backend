package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
)

const imagePath = "test/cmd/images/"

var (
	// Realistic Catalan names
	firstNames = []string{
		"Marc", "Pau", "Joan", "Josep", "Jordi",
		"Maria", "Anna", "Laura", "Montserrat", "Núria",
		"Gerard", "Carles", "Xavier", "Albert", "David",
		"Clara", "Marta", "Laia", "Emma", "Julia",
	}
	lastNames = []string{
		"Garcia", "Martinez", "López", "Serra", "Ferrer",
		"Puig", "Vila", "Soler", "Roca", "Costa",
		"Vidal", "Mas", "Roig", "Pujol", "Font",
	}

	// Realistic tool categories with descriptions
	tools = []struct {
		title       string
		description string
		category    int
		cost        int
		value       int
		image       string
	}{
		{
			title:       "Trepant percutor professional",
			description: "Trepant percutor Bosch Professional GSB 18V-50. Perfecte per treballs de bricolatge i construcció.",
			category:    1,
			cost:        15,
			value:       180,
			image:       "trepant.jpg",
		},
		{
			title:       "Serra circular de mà",
			description: "Serra circular Makita 165mm 1200W. Ideal per tallar fusta amb precisió.",
			category:    1,
			cost:        20,
			value:       150,
			image:       "serra.jpeg",
		},
		{
			title:       "Desbrossadora gasolina",
			description: "Desbrossadora Stihl FS 55. Perfecta per mantenir el jardí net.",
			category:    2,
			cost:        25,
			value:       500,
			image:       "desbrossadora.jpeg",
		},
		{
			title:       "Escala extensible alumini",
			description: "Escala extensible 3x3m. Molt lleugera i fàcil de transportar.",
			category:    3,
			cost:        10,
			value:       220,
			image:       "escala.jpg",
		},
		{
			title:       "Carretó de mà",
			description: "Carretó metàl·lic amb roda pneumàtica. Capacitat 80L.",
			category:    3,
			cost:        8,
			value:       60,
			image:       "carreto.jpeg",
		},
	}

	// Realistic locations in Catalonia (latitude, longitude)
	locations = []struct {
		lat  float64
		long float64
		city string
	}{
		{41.3879, 2.1699, "Barcelona"},
		{41.9794, 2.8214, "Girona"},
		{41.6175, 0.6200, "Lleida"},
		{41.1188, 1.2445, "Tarragona"},
		{41.5488, 2.4408, "Granollers"},
		{41.5428, 2.1044, "Sabadell"},
		{41.5463, 2.0943, "Terrassa"},
		{41.2279, 1.8038, "Vilanova i la Geltrú"},
	}

	// Realistic rating comments
	positiveComments = []string{
		"Molt bona experiència, l'eina estava en perfecte estat",
		"Excel·lent tracte i comunicació",
		"Tot perfecte, molt recomanable",
		"Eina en molt bon estat i el propietari molt amable",
		"Molt professional i puntual",
	}
	negativeComments = []string{
		"L'eina no estava en tan bon estat com s'indicava",
		"Una mica complicat coordinar l'entrega",
		"Funcionava bé però estava bastant desgastada",
		"El preu una mica elevat pel tipus d'eina",
		"La comunicació podria haver estat millor",
	}
)

func main() {
	apiURL := flag.String("api", "http://localhost:3333", "API URL")
	numUsers := flag.Int("users", 10, "Number of users to create")
	flag.Parse()

	log.Printf("Starting test with API URL: %s", *apiURL)

	// Create test service
	s := &testService{
		url: *apiURL,
		c:   &http.Client{},
	}

	// Create users
	users := make([]user, *numUsers)
	for i := 0; i < *numUsers; i++ {
		firstName := firstNames[rand.Intn(len(firstNames))]
		lastName := lastNames[rand.Intn(len(lastNames))]
		name := fmt.Sprintf("%s %s", firstName, lastName)
		userName := fmt.Sprintf("%s%d", strings.ToLower(firstName), rand.Intn(1000))
		email := fmt.Sprintf("%s@test.cat", userName)
		password := userName
		loc := locations[rand.Intn(len(locations))]

		log.Printf("Creating user: %s (%s) from %s", name, email, loc.city)
		jwt, userID := s.registerAndLogin(email, name, password, loc.lat, loc.long)
		users[i] = user{
			jwt:     jwt,
			id:      userID,
			name:    name,
			email:   email,
			toolIDs: make([]int64, 0),
		}
	}

	// Create tools for each user
	for i := range users {
		numTools := rand.Intn(3) + 1 // 1-3 tools per user
		for j := 0; j < numTools; j++ {
			baseTool := tools[rand.Intn(len(tools))]
			loc := locations[rand.Intn(len(locations))]

			// Make title unique by appending user's name and random number
			uniqueTitle := fmt.Sprintf("%s - %s (%d)", baseTool.title, users[i].name, rand.Intn(1000))

			// Create more detailed description
			detailedDesc := fmt.Sprintf(
				"%s\n\n"+
					"Característiques:\n"+
					"- Marca: %s\n"+
					"- Model: %s\n"+
					"- Antiguitat: %d anys\n"+
					"- Estat: %s\n\n"+
					"Inclou:\n"+
					"- Manual d'usuari\n"+
					"- Maleta de transport\n"+
					"- Accessoris bàsics",
				baseTool.description,
				[]string{"Bosch", "Makita", "DeWalt", "Milwaukee", "Stihl"}[rand.Intn(5)],
				[]string{"Professional", "Expert", "Plus", "Max", "Ultra"}[rand.Intn(5)],
				rand.Intn(3)+1,
				[]string{"Com nou", "Bon estat", "Estat acceptable"}[rand.Intn(3)],
			)

			// Upload an image first
			imageHash := s.uploadToolImage(users[i].jwt, baseTool.title, path.Join(imagePath, baseTool.image))

			log.Printf("Creating tool '%s' for user %s in %s", uniqueTitle, users[i].name, loc.city)
			toolID := s.createTool(users[i].jwt, struct {
				title       string
				description string
				category    int
				cost        int
				value       int
			}{
				title:       uniqueTitle,
				description: detailedDesc,
				category:    baseTool.category,
				cost:        baseTool.cost,
				value:       baseTool.value,
			}, loc.lat, loc.long, imageHash)
			users[i].toolIDs = append(users[i].toolIDs, toolID)
		}
	}

	// Create bookings between users
	for i := range users {
		// Each user makes 1-2 booking requests
		numBookings := rand.Intn(2) + 1
		for j := 0; j < numBookings; j++ {
			// Find a random tool from another user
			targetUser := rand.Intn(len(users))
			for targetUser == i || len(users[targetUser].toolIDs) == 0 {
				targetUser = rand.Intn(len(users))
			}
			toolID := users[targetUser].toolIDs[rand.Intn(len(users[targetUser].toolIDs))]

			// Create booking request
			startDate := time.Now().Add(time.Duration(rand.Intn(30)) * 24 * time.Hour)
			endDate := startDate.Add(time.Duration(rand.Intn(3)+1) * 24 * time.Hour)

			log.Printf("Creating booking request from %s to %s", users[i].name, users[targetUser].name)
			bookingID := s.createBooking(users[i].jwt, toolID, startDate.Unix(), endDate.Unix())

			// Owner accepts or rejects booking (80% acceptance rate)
			if rand.Float32() < 0.8 {
				log.Printf("Accepting booking request from %s", users[i].name)
				s.acceptBooking(users[targetUser].jwt, bookingID)

				// Mark as returned after end date (if in past)
				if endDate.Before(time.Now()) {
					log.Printf("Marking booking as returned")
					s.returnBooking(users[targetUser].jwt, bookingID)

					// Add rating (90% chance)
					if rand.Float32() < 0.9 {
						rating := rand.Intn(3) + 3 // 3-5 stars (mostly positive)
						comment := positiveComments[rand.Intn(len(positiveComments))]
						if rating < 4 {
							comment = negativeComments[rand.Intn(len(negativeComments))]
						}
						log.Printf("Adding %d-star rating: %s", rating, comment)
						s.rateBooking(users[i].jwt, bookingID, rating, comment)
					}
				}
			} else {
				log.Printf("Rejecting booking request from %s", users[i].name)
				s.denyBooking(users[targetUser].jwt, bookingID)
			}
		}
	}

	log.Println("Test completed successfully")
}

type user struct {
	jwt     string
	id      string
	name    string
	email   string
	toolIDs []int64
}

type testService struct {
	url string
	c   *http.Client
}

func (s *testService) registerAndLogin(email, name, password string, lat, long float64) (string, string) {
	// Register
	_, err := s.request(http.MethodPost, "",
		&api.Register{
			UserEmail:         email,
			RegisterAuthToken: utils.RegisterToken,
			UserProfile: api.UserProfile{
				Name:      name,
				Community: "testCommunity",
				Password:  password,
				Location: &api.Location{
					Latitude:  int64(lat * 1e6),
					Longitude: int64(long * 1e6),
				},
			},
		},
		"register",
	)
	if err != nil {
		log.Fatalf("Failed to register user: %v", err)
	}

	// Login
	loginResp, err := s.request(http.MethodPost, "",
		&api.Login{
			Email:    email,
			Password: password,
		},
		"login",
	)
	if err != nil {
		log.Fatalf("Failed to login user: %v", err)
	}

	var loginResponse struct {
		Data api.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(loginResp, &loginResponse); err != nil {
		log.Fatalf("Failed to parse login response: %v", err)
	}
	jwt := loginResponse.Data.Token

	// Get profile to get user ID
	profileResp, err := s.request(http.MethodGet, jwt, nil, "profile")
	if err != nil {
		log.Fatalf("Failed to get user profile: %v", err)
	}

	var profileResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(profileResp, &profileResponse); err != nil {
		log.Fatalf("Failed to parse profile response: %v", err)
	}

	return jwt, profileResponse.Data.ID
}

func (s *testService) uploadToolImage(jwt, toolName string, imgpath string) string {
	// Load image data
	imageData, err := os.ReadFile(imgpath)
	if err != nil {
		log.Fatalf("Failed to decode base64 image data: %v", err)
	}

	// Upload image
	resp, err := s.request(http.MethodPost, jwt,
		map[string]interface{}{
			"content": imageData,
			"name":    fmt.Sprintf("%s.jpg", toolName),
		},
		"images",
	)
	if err != nil {
		log.Fatalf("Failed to upload image: %v", err)
	}

	var response struct {
		Data struct {
			Hash string `json:"hash"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &response); err != nil {
		log.Fatalf("Failed to parse image upload response: %v", err)
	}
	return response.Data.Hash
}

func (s *testService) createTool(
	jwt string,
	tool struct {
		title       string
		description string
		category    int
		cost        int
		value       int
	},
	lat, long float64,
	imageHash string,
) int64 {
	resp, err := s.request(http.MethodPost, jwt,
		map[string]interface{}{
			"title":          tool.title,
			"description":    tool.description,
			"mayBeFree":      true,
			"askWithFee":     false,
			"cost":           tool.cost,
			"category":       tool.category,
			"estimatedValue": tool.value,
			"height":         30,
			"weight":         40,
			"images":         []string{imageHash},
			"location": map[string]interface{}{
				"latitude":  int64(lat * 1e6),
				"longitude": int64(long * 1e6),
			},
		},
		"tools",
	)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &response); err != nil {
		log.Fatalf("Failed to parse create tool response: %v", err)
	}
	return response.Data.ID
}

func (s *testService) createBooking(jwt string, toolID int64, startDate, endDate int64) string {
	resp, err := s.request(http.MethodPost, jwt,
		map[string]interface{}{
			"toolId":    fmt.Sprint(toolID),
			"startDate": startDate,
			"endDate":   endDate,
			"contact":   "test@example.com",
			"comments":  "Test booking",
		},
		"bookings",
	)
	if err != nil {
		log.Fatalf("Failed to create booking: %v", err)
	}

	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &response); err != nil {
		log.Fatalf("Failed to parse create booking response: %v", err)
	}
	return response.Data.ID
}

func (s *testService) acceptBooking(jwt, bookingID string) {
	_, err := s.request(http.MethodPost, jwt, nil, "bookings", "petitions", bookingID, "accept")
	if err != nil {
		log.Fatalf("Failed to accept booking: %v", err)
	}
}

func (s *testService) denyBooking(jwt, bookingID string) {
	_, err := s.request(http.MethodPost, jwt, nil, "bookings", "petitions", bookingID, "deny")
	if err != nil {
		log.Fatalf("Failed to deny booking: %v", err)
	}
}

func (s *testService) returnBooking(jwt, bookingID string) {
	_, err := s.request(http.MethodPost, jwt, nil, "bookings", bookingID, "return")
	if err != nil {
		log.Fatalf("Failed to return booking: %v", err)
	}
}

func (s *testService) rateBooking(jwt, bookingID string, rating int, comment string) {
	_, err := s.request(http.MethodPost, jwt,
		map[string]interface{}{
			"rating":  rating,
			"comment": comment,
		},
		"bookings", bookingID, "rate",
	)
	if err != nil {
		log.Fatalf("Failed to rate booking: %v", err)
	}
}

func (s *testService) request(method, jwt string, jsonBody any, urlPath ...string) ([]byte, error) {
	var body []byte
	var err error
	if jsonBody != nil {
		body, err = json.Marshal(jsonBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	u, err := url.Parse(s.url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Handle the case where the last path component contains query parameters
	lastIndex := len(urlPath) - 1
	if lastIndex >= 0 && strings.Contains(urlPath[lastIndex], "?") {
		parts := strings.SplitN(urlPath[lastIndex], "?", 2)
		urlPath[lastIndex] = parts[0]
		u.Path = path.Join(u.Path, path.Join(urlPath...))
		u.RawQuery = parts[1]
	} else {
		u.Path = path.Join(u.Path, path.Join(urlPath...))
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var respBody []byte
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			err = fmt.Errorf("failed to close response body: %v (original error: %v)", cerr, err)
		}
	}()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
