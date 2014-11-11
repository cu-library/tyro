# Tyro: A Sierra API Wrapper

*Ty·ro: noun, a beginner or novice.*

[![Build Status](https://travis-ci.org/cudevmaxwell/tyro.svg?branch=master)](https://travis-ci.org/cudevmaxwell/tyro)

##Setup: 

Tyro is a standalone executable, written in Go. It should compile in Go 1.3.3. 
A web server like Nginx or Apache is not required to use it.

    ./tyro -key=yourclientkey -secret=yourclientsecret -url=yourapiurl

Tyro requires the following command line options: 

    -key= : a client key obtained from your Sierra API, doc here: https://sandbox.iii.com/docs/Default.htm#authAuth.htm
    -secret= : a client secret created for the client key
    -url= : the url for the Sierra API. (For example, the sandbox is https://sandbox.iii.com/iii/sierra-api/v1/)

These options are optional: 

    -address= : The address to serve on, passed to ListenAndServe, doc here: http://golang.org/pkg/net/http/#ListenAndServe. Defaults to ":8877". 
    -raw : If supplied, this flag will turn on access to the raw Sierra API under /raw/. 
    -acaoheader= : The origin to place in the Access-Control-Allow-Origin header.
                   Defaults to *. Is only used for the /status/bib/[bibID] and /status/item/[itemID] endpoints. 
                   Multiple origins can be supplied, delimit with the ; character. 
                   Examples: 
                   -acaoheader="http://localhost:8000" 
                   -acaoheader="http://librarywebsite.com;http://catalogue.library.com" 
    -certfile= : The location of the Certificate file, for HTTPS.
    -keyfile= : The location of the Private Key file, for HTTPS.
    -logfile= : Log file. By default, log messages will be printed to stout.
    -logmaxage= : The maximum number of days to retain old log files, in days.
    -logmaxbackups= : The maximum number of old log files to keep.
    -logmaxsize= : The maximum size of log files before they are rotated, in megabytes.
    -loglevel= : The log level. One of error, warn, info, debug, or trace. 

These flags can also be supplied by environment variables:

    TYRO_ADDRESS, TYRO_KEY, TYRO_SECRET, TYRO_URL, TYRO_RAW
    TYRO_CERTFILE, TYRO_KEYFILE, TYRO_ACAOHEADER, 
    TYRO_LOGLEVEL, TYRO_LOGFILE, TYRO_LOGMAXAGE, TYRO_LOGMAXBACKUPS, TYRO_LOGMAXSIZE

This [Twelve-Factor](http://12factor.net/) style should make it easy to daemonize or Docker-ize this app. 
The TYRO_RAW environment variable, if set, should be True or False.
Log rolling is provided by [lumberjack](http://github.com/natefinch/lumberjack).

#Usage

Tyro provides the following URLs (endpoints?, routes?)

    / : Home page, HTML
    /status/bib/[bibID] : Status JSON, returns a JSON doc like:
        {
          Entries: [
            {
              CallNumber: " JC578.R383 G67 2007",
              Status: "IN LIBRARY",
              Location: "Floor 4 Books"
            }
          ]
        } 
    /status/item/[itemID] : Status JSON, returns a JSON doc like: 
        {
            CallNumber: " JC578.R383 G67 2007",
            Status: "IN LIBRARY",
            Location: "Floor 4 Books"
        }

This extra endpoint will provided if `-raw` is passed as a flag or the TYRO_RAW environment variable is set to True.

    /raw : A thin wrapper around the Sierra API. Tyro will take care of the bearer tokens and X-Forwarded-For header. 

The `/status/bib/[bibID]` and `/status/item/[itemID]` endpoints are the only ones that will respect the Access-Control-Allow-Origin header. 
If the 'raw' setting is turned on, requests sent to `/raw/` will receive whatever the Sierra API would return if the client had authenticated itself. 

This software is now in beta. Please create issues for bugs or feature requests. 

#Contributors

Joe Montibello, https://github.com/joemontibello
