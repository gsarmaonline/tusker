// testclient is a minimal local server for testing the OAuth flow end-to-end.
// It catches the post-auth redirect from Tusker and prints the user_id.
//
// Usage:
//
//	go run ./cmd/testclient
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "missing user_id")
			log.Println("OAuth callback received but user_id was empty")
			return
		}

		log.Printf("OAuth complete — user_id: %s", userID)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
<h2>OAuth complete</h2>
<p><strong>user_id:</strong> %s</p>
<p>Now fetch your token:</p>
<pre>curl "http://localhost:8080/oauth/google/token?user_id=%s" \
  -H "Authorization: Bearer YOUR_API_KEY" | jq .</pre>
</body></html>`, userID, userID)
	})

	addr := ":9999"
	log.Printf("testclient listening on %s — waiting for OAuth redirect...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
