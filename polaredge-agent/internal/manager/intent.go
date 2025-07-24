package manager

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type IngressIntent struct {
	RouteID          string            `json:"routeID"`
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	Namespace        string            `json:"namespace"`
	Annotations      map[string]string `json:"annotations"`
	Labels           map[string]string `json:"labels"`
	Path             string            `json:"path"`
	IngressClassName string            `json:"ingressClassName"`
}

type RouteStatus struct {
	RouteID   string `json:"routeID"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	Port      int    `json:"port"`
	Namespace string `json:"namespace"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func HandleIngressIntent(w http.ResponseWriter, r *http.Request) {
	var intent IngressIntent
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	status := RouteStatus{
		RouteID:   intent.RouteID,
		Mode:      "public", // default for now
		Status:    "received",
		Port:      intent.Port,
		Namespace: intent.Namespace,
		Message:   "Intent acknowledged and stored",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	statusFile := filepath.Join("status", "status.json")
	file, err := os.Create(statusFile)
	if err != nil {
		http.Error(w, "Failed to write status", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	_ = enc.Encode([]RouteStatus{status})

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(status)
}
