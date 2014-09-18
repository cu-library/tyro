package main

import (
    "flag"
    "log"
    "os"
    "net/http"
    "fmt"
    "strconv"
    "bytes"
    "encoding/json"
    "path"
    "time"
    "net/url"
    "net/http/httputil"
    "net"
    "strings"
)

//The prefix for all the curator environment variables
const EnvPrefix string = "TYRO_"

//The default address to serve from
const DefaultAddress string = ":8877"

//Will we run in verbose mode?
const DefaultVerbose bool = false

//API URL
const DefaultUrl string = "https://sandbox.iii.com/iii/sierra-api/v1/"

//API Endpoints
const TokenRequestEndpoint string = "token"
const ItemRequestEndpoint string = "items"

var (
    address = flag.String("address", DefaultAddress, "Address for the server to bind on.")
    verbose = flag.Bool("v", DefaultVerbose, "Print debugging information.")
    apiUrl = flag.String("url", DefaultUrl, "API url.")
    clientKey = flag.String("key", "", "Client Key")
    clientSecret = flag.String("secret", "", "Client Secret")

    tokenChan chan string
    refreshTokenChan chan bool 
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
 
    token := <-tokenChan            

    if token == "uninitialized" {
      http.Error(w, "Token Error, likely not yet generated.", http.StatusInternalServerError)
      return
    }

    w.Header().Set("Content-Type", "text/html")

    fmt.Fprint(w, "<html><h1>Welcome to Tyro! The Sierra API helper.</h1><div>Current token:</div>")

    fmt.Fprintf(w, "<pre>%v</pre></html>", token)

}

func statusHandler(w http.ResponseWriter, r *http.Request) {

    token := <-tokenChan            

    if token == "uninitialized" {
      http.Error(w, "Token Error, likely not yet generated.", http.StatusInternalServerError)
      return
    }


    bibID := strings.Split(r.URL.Path[len("/status/"):], "/")[0]

    if bibID == "" {
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprint(w, "<html><pre>No bibID set.</pre></html>")
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
        return
    }

    getItemStatus.Header.Add("Authorization", "Bearer " + token)
    getItemStatus.Header.Add("User-Agent", "Tyro")

    originalForwardFor := r.Header.Get("X-Forwarded-For")
    if originalForwardFor == ""{
        ip,_,_ := net.SplitHostPort(r.RemoteAddr)
        getItemStatus.Header.Add("X-Forwarded-For", ip)
    } else {
        getItemStatus.Header.Add("X-Forwarded-For", originalForwardFor)
    }

    client := &http.Client{}
    resp, err := client.Do(getItemStatus)
    if err != nil {
        logIfVerbose(resp)
        return
    }

    var responseJson struct {
        Entries [] struct {
            CallNumber string `json:"callNumber"`            
            Status struct {
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
      http.Error(w, "JSON Error", http.StatusInternalServerError)
      return
    }

    type Entry struct {
        CallNumber string 
        Status string
        Location string
    }

    var statusJson struct {
        Entries []Entry
    }

    for _,responseEntry := range responseJson.Entries {
        newEntry := Entry{}
        newEntry.CallNumber = responseEntry.CallNumber
        newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|a", " ", -1)
        newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|b", " ", -1)
        if  responseEntry.Status.DueDate.IsZero() {
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
      logIfVerbose("Token Error")
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
    
    r.Header.Add("Authorization", "Bearer " + token)
    r.Header.Add("User-Agent", "Tyro")

    originalForwardFor := r.Header.Get("X-Forwarded-For")
    if originalForwardFor == ""{
        ip,_,_ := net.SplitHostPort(r.RemoteAddr)
        r.Header.Add("X-Forwarded-For", ip)
    } else {
        r.Header.Add("X-Forwarded-For", originalForwardFor)
    }

    logIfVerbose("Sending proxied request:")
    logIfVerbose(r)

}

func tokener(){

    type AuthTokenResponse struct {
        AccessToken string `json:"access_token"`
        TokenType string `json:"token_type"`
        ExpiresIn int `json:"expires_in"`
    }

    token := "uninitialized"

    for {
        select {
            case <-refreshTokenChan:

                logIfVerbose("Asking for new token...")

                stopIntrim := make(chan bool)

                go func() {
                    logIfVerbose("Serving old token while we wait.")
                    RunForever:
                    for{
                        select {
                            case tokenChan <- token:
                                logIfVerbose("Sent token: " + token)
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
                    return
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
                }

                logIfVerbose(responseJson)

                stopIntrim <- true
                <- stopIntrim

                token = responseJson.AccessToken

                logIfVerbose("Received new token from API.")

                go func() {
                    time.Sleep(time.Duration(responseJson.ExpiresIn - 20) * time.Second)
                    refreshTokenChan <- true
                }()

            case tokenChan <- token:
                logIfVerbose("Sent token: " + token)
        }
    }
}

func main() {

    flag.Usage = func() {
        fmt.Print("Tyro: A helper for Sierra APIs\n\n")
        flag.PrintDefaults()
        fmt.Println("The possible environment variables:")
        fmt.Println("TYRO_ADDRESS, TYRO_VERBOSE, TYRO_KEY, TYRO_SECRET, TYRO_URL")
    }

    flag.Parse()

    //Look at the environment variables.
    fromEnv() 

    logIfVerbose("Serving on address: " + *address)
    logIfVerbose("Using Client Key: " + *clientKey)
    logIfVerbose("Using Client Secret: " + *clientSecret)
    logIfVerbose("Connecting to API URL: " + *apiUrl)

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
    err := http.ListenAndServe(*address, nil)
    if err != nil {
        log.Fatal(err)
    }

}

//Utility Functions

func fromEnv() {
    getAddressFromEnvOrDefault()
    getVerboseFromEnvOrDefault()
    getUrlFromEnvOrDefault()
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
                log.Fatalf("Unable to parse '%v' (%v) as boolean.", envVerboseString, EnvPrefix + "VERBOSE")
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


func getClientKeyFromEnvOrFail() {
    if *clientKey == "" {
        envClientKey := os.Getenv(EnvPrefix + "KEY")
        if envClientKey != "" {
            *clientKey = envClientKey
        } else {
            log.Fatalf("Unable to find Client Key in environment variable %v or provided by flag -key=", 
                       EnvPrefix + "KEY")
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
                       EnvPrefix + "Secret")
        }
    }
}

//Log a message if the verbose flag is set.
func logIfVerbose(message interface{}) {
    if *verbose {
        log.Println(message)
    }
}

