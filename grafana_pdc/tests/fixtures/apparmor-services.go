// Local-only Supervisor and PDC signing endpoints for the enforcing image test.
// The container has no external network and uses disposable test keys.
package main

import (
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

func main() {
	if err := os.MkdirAll("/tmp/pdc-sign", 0700); err != nil {
		log.Fatal(err)
	}
	var signing sync.Mutex
	supervisor := http.NewServeMux()
	supervisor.HandleFunc("/addons/self/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"ok","data":{"slug":"test_grafana_pdc","name":"PDC test","version":"test"}}`))
	})
	supervisor.HandleFunc("/services/mqtt", func(w http.ResponseWriter, _ *http.Request) {
		log.Print("fixture: reporter requested MQTT service")
		http.Error(w, "no broker in this fixture", http.StatusNotFound)
	})
	supervisor.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	go func() {
		log.Fatal((&http.Server{Addr: "127.0.0.1:18080", Handler: supervisor, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
	}()
	signer := http.NewServeMux()
	signer.HandleFunc("/pdc/api/v1/sign-public-key", func(w http.ResponseWriter, r *http.Request) {
		signing.Lock()
		defer signing.Unlock()
		var request struct {
			PublicKey string `json:"publicKey"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&request); err != nil || request.PublicKey == "" {
			http.Error(w, "invalid signing request", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile("/tmp/pdc-sign/client.pub", []byte(request.PublicKey), 0600); err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("ssh-keygen", "-q", "-s", "/tmp/pdc-fixture/ssh-ca", "-I", "apparmor-test", "-n", "test", "-V", "-1m:+1h", "/tmp/pdc-sign/client.pub")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("sign test key: %v: %s", err, output)
		}
		cert, err := os.ReadFile("/tmp/pdc-sign/client-cert.pub")
		if err != nil {
			log.Fatal(err)
		}
		hostKey, err := os.ReadFile("/tmp/pdc-fixture/ssh-ca.pub")
		if err != nil {
			log.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"certificate": string(pem.EncodeToMemory(&pem.Block{Type: "SSH CERTIFICATE", Bytes: cert})), "known_hosts": "gateway.example.test " + string(hostKey)}); err != nil {
			log.Print(err)
		}
		log.Print("fixture: signed PDC public key")
	})
	log.Fatal((&http.Server{Addr: "127.0.0.1:443", Handler: signer, ReadHeaderTimeout: 5 * time.Second}).ListenAndServeTLS("/tmp/pdc-fixture/tls.crt", "/tmp/pdc-fixture/tls.key"))
}
