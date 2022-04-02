package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func main() {

	clientId,ok := os.LookupEnv("CLIENT_ID")
	if !ok {
		log.Fatal("CLIENT_ID is not set")
	} else {
		log.Printf("CLIENT_ID: %v\n", clientId)
	}

	clientSecret, ok := os.LookupEnv("CLIENT_SECRET")
	if !ok {
		log.Fatal("CLIENT_SECRET is not set")
	} else {
		log.Printf("CLIENT_SECRET: %v\n", clientSecret)
	}

	tenantId,ok := os.LookupEnv("TENANT_ID")
	if !ok {
		log.Fatal("TENANT_ID is not present")
	} else {
		log.Printf("TENANT_ID: %v\n", tenantId)
	}

	callbackUrl,ok := os.LookupEnv("CALLBACK_URL")
	if !ok {
		log.Fatal("CALLBACK_URL is not present")
	} else {
		log.Printf("CALLBACK_URL: %v\n", callbackUrl)
	}

	cookieDomain,ok := os.LookupEnv("COOKIE_DOMAIN")
	if !ok {
		log.Fatal("COOKIE_DOMAIN is not present")
	} else {
		log.Printf("COOKIE_DOMAIN: %v\n", cookieDomain)
	}

	port,ok := os.LookupEnv("LISTEN_PORT")
	if !ok {

		port="8080"
	}
    
	log.Printf("LISTEN_PORT: %v\n", port)

	ctx := context.Background()

	// TODO make it more generic

	issuser_uri:=fmt.Sprintf("https://sts.windows.net/%s/", tenantId)

	provider, err := oidc.NewProvider(ctx, issuser_uri)
	if err != nil {
		log.Printf("NewProvider: " + issuser_uri)
		log.Fatal(err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientId})
	config := oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  callbackUrl,
		// TODO Scopes should be customized
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

    //  for liveness probe in Kubernetes
	http.HandleFunc("/healthz", healthz)
    //  for readiness probe in Kubernetes
    http.HandleFunc("/readyz", readyz)


	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("id_token")
		if err != nil {
			log.Println("home handler, unable to retrieve id_token cookie: " + err.Error())

			// TODO: its not an error - render home page html for anonymous user
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idToken, err := verifier.Verify(ctx, cookie.Value)
		if err != nil {
			log.Println("home handler, unable to verify id_token: " + err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// TODO: render html page
		user := User{}
		idToken.Claims(&user)
		data, err := json.Marshal(user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})

	http.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("id_token")
		if err != nil {
			log.Println("check handler, unable to get id_token cookis: " + err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idToken, err := verifier.Verify(ctx, cookie.Value)
		if err != nil {
			log.Println("check handler, unable to verify id token: " + err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user := User{}
		idToken.Claims(&user)
		log.Println("check handler, success: " + user.Email)

		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		rd := r.URL.Query().Get("rd")
		if rd == "" {
			rd = "/"
		}

		state, err := randString(16)
		if err != nil {
			log.Println("login handler, unable create state: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		nonce, err := randString(16)
		if err != nil {
			log.Println("login handler, unable create nonce: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		ttl := int((5 * time.Minute).Seconds())

		setCallbackCookie(w, r, "rd", rd, cookieDomain, ttl)
		setCallbackCookie(w, r, "state", state, cookieDomain, ttl)
		setCallbackCookie(w, r, "nonce", nonce, cookieDomain, ttl)

		log.Println("login handler, rd: " + rd)

		url := config.AuthCodeURL(state, oidc.Nonce(nonce))
		log.Println("login handler, redirecting to: " + url)
		http.Redirect(w, r, url, http.StatusFound)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		state, err := r.Cookie("state")
		if err != nil {
			log.Println("callback handler, unable to get state from cookie: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "state not found", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("state") != state.Value {
			log.Println("callback handler, state from cookie and identity provider did not match")
			// TODO: user facing page, need html representation
			http.Error(w, "state did not match", http.StatusBadRequest)
			return
		}

		oauth2Token, err := config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			log.Println("callback handler, unable to exchange code for access token: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			log.Println("callback handler, unable to get id_token from oauth2 token")
			// TODO: user facing page, need html representation
			http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
			return
		}

		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			log.Println("callback handler, unable to verify id_token: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		nonce, err := r.Cookie("nonce")
		if err != nil {
			log.Println("callback handler, unable get nonce from cookie: " + err.Error())
			// TODO: user facing page, need html representation
			http.Error(w, "nonce not found", http.StatusBadRequest)
			return
		}
		if idToken.Nonce != nonce.Value {
			log.Println("callback handler, nonce in cookie and id_token did not match")
			// TODO: user facing page, need html representation
			http.Error(w, "nonce did not match", http.StatusBadRequest)
			return
		}

		user := User{}
		idToken.Claims(&user)

		setCallbackCookie(w, r, "id_token", rawIDToken, cookieDomain, int(time.Until(oauth2Token.Expiry).Seconds()))

		log.Println("callback handler, successfully logged in " + user.Email)

		rd, err := r.Cookie("rd")
		if err != nil || rd.Value == "" {
			rd.Value = "/"
		}

		http.Redirect(w, r, rd.Value, http.StatusFound)
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		setCallbackCookie(w, r, "id_token", "", cookieDomain, 0)

		rd := r.URL.Query().Get("rd")
		if rd == "" {
			rd = "/"
		}

		http.Redirect(w, r, rd, http.StatusFound)
	})

	log.Println("listening on http://0.0.0.0"+port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s",port), nil))
}

type User struct {
	// Id    string   `json:"sub"`
	Name  string   `json:"name"`
	Email string   `json:"unique_name"` // unique_name, upn
	Roles []string `json:"roles`
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value, domain string, ttl int) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   domain,
		MaxAge:   ttl,
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

// healthz is a liveness probe.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
func readyz(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration((5)) * time.Second)
	fmt.Fprintf(w, "ok")
	}


