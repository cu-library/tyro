# Tyro: A Sierra API Wrapper

*Ty·ro: noun, a beginner or novice.*

##Setup: 

Tyro is a standalone executable, written in Go. It should compile in Go 1.3. A web server like Nginx or Apache is not requred to use it.

    ./tyro -key=yourclientkey -secret=yourclientsecret -url=yourapiurl

Tyro requires the following command line options: 

    -key= : a client key obtained from your Sierra API, doc here: https://sandbox.iii.com/docs/Default.htm#authAuth.htm
    -secret= : a client secret created for the client key
    -url= : the url for the Sierra API. (For example, the sandbox is https://sandbox.iii.com/iii/sierra-api/v1/)

These options are optional: 

    -address= : The address to serve on, passed to ListenAndServe, doc here: http://golang.org/pkg/net/http/#ListenAndServe. Defaults to ":8877". 
    -v : The verbose flag. Prints debugging information.

These flags can also be supplied by environment variables:

    TYRO_ADDRESS, TYRO_VERBOSE, TYRO_KEY, TYRO_SECRET, TYRO_URL

This [Twelve-Factor](http://12factor.net/) style should make it easy to deamonize or Docker-ize this app. 

#Usage

Tyro provides the following URLs (endpoints?, routes?)

    / : Home page, HTML
    /status/[bibID] : Status JSON, returns a JSON doc like:
        {
          Entries: [
            {
              CallNumber: " JC578.R383 G67 2007",
              Status: "IN LIBRARY",
              Location: "Floor 4 Books"
            }
          ]
        } 
    /raw : A thin wrapper around the raw Sierra API. Tyro will take care of the bearer tokens and X-Forwarded-For header. 
    /static[/file] : Serves any files in a directory named 'static' next to the executable. Used mostly for testing and demonstration purposes. Automatically serves an index.html if you visit /static. 

This software is still in alpha, and is more of a test of writing in Go and using the Sierra API. Upcoming features:

1. CORS support, to let it be used as a public API. 
2. Cleaning up debugging, better logging.
3. Tests and CI. 
4. Things your create an issue for! 
