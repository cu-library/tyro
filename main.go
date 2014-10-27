package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

//The prefix for all the curator environment variables
const EnvPrefix string = "TYRO_"

//The default address to serve from
const DefaultAddress string = ":8877"

//Will we run in verbose mode?
const DefaultVerbose bool = false

const DefaultCertFile string = ""
const DefaultKeyFile string = ""

//API URL
const DefaultUrl string = "https://sandbox.iii.com/iii/sierra-api/v1/"

//API Endpoints
const TokenRequestEndpoint string = "token"
const ItemRequestEndpoint string = "items"

var (
	address      = flag.String("address", DefaultAddress, "Address for the server to bind on.")
	verbose      = flag.Bool("v", DefaultVerbose, "Print debugging information.")
	apiUrl       = flag.String("url", DefaultUrl, "API url.")
	certFile     = flag.String("certfile", DefaultCertFile, "Certificate file location.")
	keyFile      = flag.String("keyfile", DefaultKeyFile, "Private key file location.")
	clientKey    = flag.String("key", "", "Client Key")
	clientSecret = flag.String("secret", "", "Client Secret")

	templates = template.Must(template.ParseGlob("templates/*.html"))

	tokenChan        chan string
	refreshTokenChan chan bool
)

func main() {

	flag.Usage = func() {
		fmt.Print("Tyro: A helper for Sierra APIs\n\n")
		flag.PrintDefaults()
		fmt.Println("The possible environment variables:")
		fmt.Println("TYRO_ADDRESS, TYRO_VERBOSE, TYRO_KEY, TYRO_SECRET, TYRO_URL, TYRO_CERT_FILE, TYRO_KEY_FILE")
		fmt.Println("If a certificate file is provided, Tyro will attempt to use HTTPS.")
	}

	flag.Parse()

	//Look at the environment variables.
	fromEnv()

	logIfVerbose("Serving on address: " + *address)
	logIfVerbose("Using Client Key: " + *clientKey)
	logIfVerbose("Using Client Secret: " + *clientSecret)
	logIfVerbose("Connecting to API URL: " + *apiUrl)

	if *certFile != DefaultCertFile {
		logIfVerbose("Going to try to serve through HTTPS")
		logIfVerbose("Using Certificate File: " + *certFile)
		logIfVerbose("Using Private Key File: " + *keyFile)
	}

	tokenChan = make(chan string)
	refreshTokenChan = make(chan bool)
	go tokener()
	refreshTokenChan <- true

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status/", statusHandler)
	rawProxy := httputil.NewSingleHostReverseProxy(&url.URL{})
	rawProxy.Director = rawRewriter
	http.Handle("/raw/", rawProxy)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	if *certFile == DefaultCertFile {
		log.Fatal(http.ListenAndServe(*address, nil))
	} else {
		//Remove SSL 3.0 compatibility for POODLE exploit mitigation
		config := &tls.Config{MinVersion: tls.VersionTLS10}
		server := &http.Server{Addr: *address, Handler: nil, TLSConfig: config}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	}

}

func homeHandler(w http.ResponseWriter, r *http.Request) {

	token := <-tokenChan

	if token == "uninitialized" {
		http.Error(w, "Token Error, likely not yet generated.", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "home", nil)

}

func statusHandler(w http.ResponseWriter, r *http.Request) {

	token := <-tokenChan

	if token == "uninitialized" {
		http.Error(w, "Token Error, token not yet created.", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, token not yet generated.")
		return
	}

	if token == "" {
		http.Error(w, "Token Error, token creation failed.", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, token creation failed.")
		return
	}

	bibID := strings.Split(r.URL.Path[len("/status/"):], "/")[0]

	if bibID == "" {
		http.Error(w, "Error, you need to provide a Bib ID. /status/[BidID]", http.StatusBadRequest)
		logIfVerbose("Bad Request at /status/ handler, no BidID provided.")
		return
	}

	parsedApiUrl, err := url.Parse(*apiUrl)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	itemStatusURL := parsedApiUrl
	itemStatusURL.Path = path.Join(itemStatusURL.Path, ItemRequestEndpoint)

	q := itemStatusURL.Query()
	q.Set("bibIds", bibID)
	q.Set("deleted", "false")
	itemStatusURL.RawQuery = q.Encode()

	getItemStatus, err := http.NewRequest("GET", itemStatusURL.String(), nil)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	setAuthorizationHeaders(getItemStatus, r, token)

	client := &http.Client{}
	resp, err := client.Do(getItemStatus)
	if err != nil {
		http.Error(w, "Error querying Sierra API", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, GET against itemStatusURL failed.")
		logIfVerbose(err)
		return
	}

    if resp.StatusCode == 401 {
        http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
        logIfVerbose("Internal Server Error at /status/ handler, token is out of date.")
        refreshTokenChan <- true
        return
    }

	var responseJson struct {
		Entries []struct {
			CallNumber string `json:"callNumber"`
			Status     struct {
				DueDate time.Time `json:"duedate"`
			} `json:"status"`
			Location struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"entries"`
	}

	err = json.NewDecoder(resp.Body).Decode(&responseJson)
	defer resp.Body.Close()
	if err != nil {
		http.Error(w, "JSON Decoding Error", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, JSON Decoding Error")
		logIfVerbose(err)
		return
	}

	type Entry struct {
		CallNumber string
		Status     string
		Location   string
	}

	var statusJson struct {
		Entries []Entry
	}

	for _, responseEntry := range responseJson.Entries {
		newEntry := Entry{}
		newEntry.CallNumber = responseEntry.CallNumber
		newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|a", " ", -1)
		newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|b", " ", -1)
		if responseEntry.Status.DueDate.IsZero() {
			newEntry.Status = "IN LIBRARY"
		} else {
			newEntry.Status = "DUE " + responseEntry.Status.DueDate.Format("January 2, 2006")
		}
		newEntry.Location = responseEntry.Location.Name

		statusJson.Entries = append(statusJson.Entries, newEntry)
	}

	json, err := json.MarshalIndent(statusJson, "", "   ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)

}

func rawRewriter(r *http.Request) {

	token := <-tokenChan

	if token == "uninitialized" {
		logIfVerbose("Error at /raw/ handler, token not yet generated.")
		return
	}

	if token == "" {
		logIfVerbose("Error at /raw/ handler, token creation failed.")
		return
	}

	parsedApiUrl, err := url.Parse(*apiUrl)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	rawRequestURL := parsedApiUrl
	rawRequestURL.Path = path.Join(rawRequestURL.Path, r.URL.Path[len("/raw/"):])
	rawRequestURL.RawQuery = r.URL.RawQuery

	r.URL = rawRequestURL

	setAuthorizationHeaders(r, r, token)

	logIfVerbose("Sending proxied request:")
	logIfVerbose(r)

}

func tokener() {

	type AuthTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	token := "uninitialized"

	for {
		select {
		case <-refreshTokenChan:

			logIfVerbose("Asking for new token...")

			stopIntrim := make(chan bool)

			go func() {
				logIfVerbose("Serving old token while we wait.")
				oldToken := token
			RunForever:
				for {
					select {
					case tokenChan <- oldToken:
						logIfVerbose("Sent token: " + oldToken)
					case <-stopIntrim:
						close(stopIntrim)
						break RunForever
					}
				}
			}()

			parsedApiUrl, err := url.Parse(*apiUrl)
			if err != nil {
				//No recovery possible here, probable problem with URL
				log.Fatal(err)
			}

			tokenRequestURL := parsedApiUrl
			tokenRequestURL.Path = path.Join(tokenRequestURL.Path, TokenRequestEndpoint)

			bodyValues := url.Values{}
			bodyValues.Set("grant_type", "client_credentials")

			getTokenRequest, err := http.NewRequest("POST", tokenRequestURL.String(), bytes.NewBufferString(bodyValues.Encode()))
			if err != nil {
				//No recovery possible here, probable problem with URL
				log.Fatal(err)
			}

			getTokenRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			getTokenRequest.SetBasicAuth(*clientKey, *clientSecret)

			client := &http.Client{}
			resp, err := client.Do(getTokenRequest)
			if err != nil {
				token = ""
				logIfVerbose("Unable to get new token!")
				logIfVerbose(err)
				logIfVerbose(resp)
				return
			}

			var responseJson AuthTokenResponse

			err = json.NewDecoder(resp.Body).Decode(&responseJson)
			defer resp.Body.Close()
			if err != nil {
				token = ""
				logIfVerbose("Unable to parse new token response!")
				logIfVerbose(err)
				logIfVerbose(resp)
				return
			}

			logIfVerbose(responseJson)

			stopIntrim <- true
			<-stopIntrim

			token = responseJson.AccessToken

			logIfVerbose("Received new token from API.")

			go func() {
				time.Sleep(time.Duration(responseJson.ExpiresIn-20) * time.Second)
				refreshTokenChan <- true
			}()

		case tokenChan <- token:
			logIfVerbose("Sent token: " + token)
		}
	}
}

//Utility Functions

func fromEnv() {
	getAddressFromEnvOrDefault()
	getVerboseFromEnvOrDefault()
	getUrlFromEnvOrDefault()
    getCertFileFromEnvOrDefault()
    getKeyFileFromEnvOrDefault()
	getClientKeyFromEnvOrFail()
	getClientSecretFromEnvOrFail()    
}

//If the address is not set on the command line, get it from the
//environment or use the default.
func getAddressFromEnvOrDefault() {
	if *address == DefaultAddress {
		envAddress := os.Getenv(EnvPrefix + "ADDRESS")
		if envAddress != "" {
			*address = envAddress
		}
	}
}

//If verbose boolean is not set on the command line, get it from the
//environment or use the default.
func getVerboseFromEnvOrDefault() {
	if *verbose == DefaultVerbose {
		envVerboseString := os.Getenv(EnvPrefix + "VERBOSE")
		if envVerboseString != "" {
			envVerboseBool, err := strconv.ParseBool(envVerboseString)
			if err != nil {
				log.Fatalf("Unable to parse '%v' (%v) as boolean.", envVerboseString, EnvPrefix+"VERBOSE")
			} else {
				*verbose = envVerboseBool
			}
		}
	}
}

//If the API URL is not set on the command line, get it from the
//environment or use the default.
func getUrlFromEnvOrDefault() {
	if *apiUrl == DefaultUrl {
		envUrl := os.Getenv(EnvPrefix + "URL")
		if envUrl != "" {
			*apiUrl = envUrl
		}
	}
}

func getCertFileFromEnvOrDefault() {
    if *certFile == DefaultCertFile {
        envCertFile := os.Getenv(EnvPrefix + "CERT_FILE")
        if envCertFile != "" {
            *certFile = envCertFile
        }
    }
}

func getKeyFileFromEnvOrDefault() {
    if *keyFile == DefaultKeyFile {
        envKeyFile := os.Getenv(EnvPrefix + "KEY_FILE")
        if envKeyFile != "" {
            *keyFile = envKeyFile
        }
    }
}

func getClientKeyFromEnvOrFail() {
	if *clientKey == "" {
		envClientKey := os.Getenv(EnvPrefix + "KEY")
		if envClientKey != "" {
			*clientKey = envClientKey
		} else {
			log.Fatalf("Unable to find Client Key in environment variable %v or provided by flag -key=",
				EnvPrefix+"KEY")
		}
	}
}

func getClientSecretFromEnvOrFail() {
	if *clientSecret == "" {
		envClientSecret := os.Getenv(EnvPrefix + "SECRET")
		if envClientSecret != "" {
			*clientSecret = envClientSecret
		} else {
			log.Fatalf("Unable to find Client Secret in environment variable %v or provided by flag -secret=",
				EnvPrefix+"Secret")
		}
	}
}

//Log a message if the verbose flag is set.
func logIfVerbose(message interface{}) {
	if *verbose {
		log.Println(message)
	}
}

//Render an HTML template.
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//Set the required Authorization headers.
//This includes the Bearer token, the user agent, and X-Forwarded-For
func setAuthorizationHeaders(nr *http.Request, or *http.Request, t string) {
	nr.Header.Add("Authorization", "Bearer "+t)
	nr.Header.Add("User-Agent", "Tyro")

	originalForwardFor := or.Header.Get("X-Forwarded-For")
	if originalForwardFor == "" {
		ip, _, _ := net.SplitHostPort(or.RemoteAddr)
		nr.Header.Add("X-Forwarded-For", ip)
	} else {
		nr.Header.Add("X-Forwarded-For", originalForwardFor)
	}
}
