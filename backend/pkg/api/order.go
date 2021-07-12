package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"
)

type Order struct {
	Url     string    `json:"url"`
	Uuid    types.UID `json:"uuid"`
	Created time.Time `json:"created"`
	Status  []string  `json:"status"`
}

var globalStore = cache.New(24*time.Hour, 30*time.Minute)

func Set(order *Order) {
	globalStore.Set(string(order.Uuid), order, 0)
}

func Get(key string) (*Order, error) {
	val, found := globalStore.Get(key)
	if !found {
		return nil, errors.New("Order not found")
	}

	return val.(*Order), nil
}

func OrderEndpoint(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		order := Order{
			Uuid:    uuid.NewUUID(),
			Created: time.Now(),
			Status:  []string{"Order received"},
		}
		err := json.NewDecoder(r.Body).Decode(&order)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("Storing order %s", order.Uuid)

		Set(&order)
		go processOrder(&order)

		w.Header().Set("Location", fmt.Sprintf("/order/%s", order.Uuid))
		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		uuid, ok := r.URL.Query()["uuid"]
		if !ok || len(uuid[0]) < 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		klog.Infof("Fetching order %s", uuid[0])
		order, err := Get(uuid[0])
		if err != nil {
			klog.Errorf("Order get failed: %s", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if order.Uuid == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(order)
		if err != nil {
			klog.Errorf("Order get failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func processOrder(order *Order) {
	go triggerJupyterBookPipeline(order)
	go triggerImageBuildPipeline(order)
}

func triggerImageBuildPipeline(order *Order) {
	order.Status = append(order.Status, "Image build started")
}

func triggerJupyterBookPipeline(order *Order) {
	order.Status = append(order.Status, "JupyterBook build started")
}
