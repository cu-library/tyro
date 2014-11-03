# Tyro: A Sierra API Wrapper

*Ty·ro: noun, a beginner or novice.*

##Setup: 

Tyro is a standalone executable, written in Go. It should compile in Go 1.3. A web server like Nginx or Apache is not required to use it.

    ./tyro -key=yourclientkey -secret=yourclientsecret -url=yourapiurl

Tyro requires the following command line options: 

    -key= : a client key obtained from your Sierra API, doc here: https://sandbox.iii.com/docs/Default.htm#authAuth.htm
    -secret= : a client secret created for the client key
    -url= : the url for the Sierra API. (For example, the sandbox is https://sandbox.iii.com/iii/sierra-api/v1/)

These options are optional: 

    -address= : The address to serve on, passed to ListenAndServe, doc here: http://golang.org/pkg/net/http/#ListenAndServe. Defaults to ":8877". 
    -acaoheader= : The origin to place in the Access-Control-Allow-Origin header. Defaults to *. Is only used for the /status/[bidID] endpoint. Multiple origins can be supplied, delimit with the ; character. Examples: -acaoheader="http://localhost:8000" -acaoheader="http://librarywebsite.com;http://catalogue.library.com" 
    -certfile= : The location of the Certificate file, for HTTPS.
    -keyfile= : The location of the Private Key file, for HTTPS.
    -logfile= : Log file. By default, log messages will be printed to Stderr.
    -logmaxage= : The maximum number of days to retain old log files, in days.
    -logmaxbackups= : The maximum number of old log files to keep.
    -logmaxsize= : The maximum size of log files before they are rotated, in megabytes.
    -loglevel= : The log level. One of error, warn, info, debug, or trace. 

These flags can also be supplied by environment variables:

    TYRO_ADDRESS, TYRO_KEY, TYRO_SECRET, TYRO_URL, 
    TYRO_CERTFILE, TYRO_KEYFILE, TYRO_ACAOHEADER, 
    TYRO_LOGLEVEL, TYRO_LOGFILE, TYRO_LOGMAXAGE, TYRO_LOGMAXBACKUPS, TYRO_LOGMAXSIZE

This [Twelve-Factor](http://12factor.net/) style should make it easy to daemonize or Docker-ize this app. Log rolling is provided by [lumberjack](http://github.com/natefinch/lumberjack).

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
    /raw : A thin wrapper around the Sierra API. Tyro will take care of the bearer tokens and X-Forwarded-For header. 
    /static[/file] : Serves any files in a directory named 'static' next to the executable. Used mostly for testing and demonstration purposes. Automatically serves an index.html if you visit /static. 

The `/status/[bidID]` endpoint is the only one that will respect the Access-Control-Allow-Origin header. Tyro doesn't allow cross domain access to the raw Sierra API. 

This software is still in alpha. Upcoming features:

1. Cleaning up debugging, better logging.
2. More tests and CI. 
3. Things your create an issue for! 

#Contributors

Joe Montibello, https://github.com/joemontibello
