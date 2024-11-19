package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type User struct {
	UserId    string `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

type RewardSelection struct {
	UserId               string `json:"userId"`
	SelectedRewardDealId string `json:"selectedRewardDealId"`
	AcceptedTnC          bool   `json:"acceptedTnC"`
}

type Reward struct {
	RewardId  string `json:"rewardId"`
	UserId    string `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

var logger *zap.Logger

var apiKey = os.Getenv("CHOREO_API_KEY")
var tokenUrl = os.Getenv("TOKEN_URL")
var loyaltyApiUrl = os.Getenv("LOYALTY_API_URL")

func HandleRewardSelection(w http.ResponseWriter, r *http.Request) {
	var selection RewardSelection

	// Decode the request body into the RewardSelection struct
	if err := json.NewDecoder(r.Body).Decode(&selection); err != nil {
		logger.Error("Failed to decode incoming reward selection data", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request payload"))
		return
	}

	logger.Info("received reward selection",
		zap.String("UserId", selection.UserId),
		zap.String("SelectedRewardDealId", selection.SelectedRewardDealId),
		zap.Bool("AcceptedTnC", selection.AcceptedTnC),
	)

	user, err := FetchUserByIdFromLoyaltyApi(selection.UserId)
	if err != nil {
		logger.Error("Failed to fetch user", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to fetch user"))
		return
	}

	err = PostRewardSelectionToVendorManagementApi(Reward{
		RewardId:  selection.SelectedRewardDealId,
		UserId:    selection.UserId,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	})

	if err != nil {
		logger.Error("unable to send reward selection to vendor management api", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("unable to send reward selection to vendor management  api"))
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("reward selection received successfully"))
}

func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alive"))
}

func ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	// Add logic here to check database connections, external services, etc.
	// If all checks pass:
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
	// If any check fails:
	// w.WriteHeader(http.StatusInternalServerError)
}

func main() {

	defer logger.Sync() // Ensure all buffered logs are written

	logger.Info("starting the reward management api (golang)...")
	logger.Info("using the following environment variables")
	logger.Info("API_KEY: " + apiKey)
	logger.Info("TOKEN_URL: " + tokenUrl)
	logger.Info("LOYALTY_API_URL: " + loyaltyApiUrl)

	r := mux.NewRouter()
	r.HandleFunc("/select-reward", HandleRewardSelection).Methods("POST")

	r.HandleFunc("/healthz", ReadinessProbe).Methods("GET") // Readiness probe
	r.HandleFunc("/livez", LivenessProbe).Methods("GET")    // Liveness probe

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		return
	}
}

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func FetchUserByIdFromLoyaltyApi(userId string) (*User, error) {
	// Construct the full URL using the base URL from the environment variable
	url := fmt.Sprintf("%s/user/%s", loyaltyApiUrl, userId)
	// Make the HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Failed to create new request", zap.Error(err))
		return nil, fmt.Errorf("failed to create new request: %v", err)
	}
	req.Header.Set("choreo-api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to fetch user", zap.String("userId", userId), zap.Error(err))
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		logger.Warn("API responded with non-200 status code", zap.Int("statusCode", resp.StatusCode))
		return nil, fmt.Errorf("API responded with status code: %d", resp.StatusCode)
	}

	// Decode the response body into the User struct
	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		logger.Error("Failed to decode user data", zap.Error(err))
		return nil, fmt.Errorf("failed to decode user data: %v", err)
	}

	logger.Info("Successfully fetched user", zap.String("userId", user.UserId))
	return &user, nil
}

func PostRewardSelectionToVendorManagementApi(reward Reward) error {
	// Marshal the Reward struct to JSON
	payload, err := json.Marshal(reward)
	if err != nil {
		logger.Error("Failed to marshal reward", zap.Error(err))
		return err
	}
	logger.Info("Successfully processed reward selection user", zap.Any("payload", payload))
	return nil
}
