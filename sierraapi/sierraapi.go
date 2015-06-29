// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package sierraapi

import (
	"fmt"
	l "github.com/cudevmaxwell/tyro/loglevel"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	//API URL
	DefaultURL string = "https://sandbox.iii.com/iii/sierra-api/v1/"

	//API Endpoints
	TokenRequestEndpoint string = "token"
	BibRequestEndpoint   string = "bibs"
	ItemRequestEndpoint  string = "items"
)

type ItemRecordIn struct {
	CallNumber string `json:"callNumber"`
	Status     struct {
		DueDate time.Time `json:"duedate"`
	} `json:"status"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
}

type ItemRecordOut struct {
	CallNumber string
	Status     string
	Location   string
}

type ItemRecordsIn struct {
	Entries []ItemRecordIn `json:"entries"`
}

type ItemRecordsOut struct {
	Entries []ItemRecordOut
}

func (in *ItemRecordIn) Convert() *ItemRecordOut {

	out := new(ItemRecordOut)
	out.CallNumber = in.CallNumber
	out.CallNumber = strings.Replace(out.CallNumber, "|a", " ", -1)
	out.CallNumber = strings.Replace(out.CallNumber, "|b", " ", -1)
	out.CallNumber = strings.TrimSpace(out.CallNumber)
	if in.Status.DueDate.IsZero() {
		out.Status = "In Library"
	} else {
		out.Status = "Due " + in.Status.DueDate.Format("January 2, 2006")
	}
	out.Location = in.Location.Name

	return out
}

func (in *ItemRecordsIn) Convert() *ItemRecordsOut {
	out := new(ItemRecordsOut)
	for _, itemRecord := range in.Entries {
		out.Entries = append(out.Entries, *itemRecord.Convert())
	}

	return out
}

type BibRecordIn struct {
	ID   int `json:"id"`
	CreatedDate time.Time `json:"createdDate"`
	Marc struct {
		Fields []struct {
			Data struct {
				Subfields []struct {
					Code string `json:"code"`
					Data string `json:"data"`
				} `json:"subfields"`
			} `json:"data"`
			Tag string `json:"tag"`
		} `json:"fields"`
		Leader string `json:"leader"`
	} `json:"marc"`
}

type BibRecordOut struct {
	BibID           int
	TitleAndAuthor  string
	ISBNs           []string
	CreatedDate     time.Time
}


type BibRecordsIn struct {
	Entries []BibRecordIn `json:"entries"`
}

type BibRecordsOut []BibRecordOut


func (records BibRecordsOut) Len() int {
	return len(records)
}
func (records BibRecordsOut) Less(i, j int) bool {
	if records[i].CreatedDate == records[j].CreatedDate {
        return records[i].BibID < records[j].BibID
	} else {
        return records[i].CreatedDate.Before(records[j].CreatedDate)
    }
}

func (records BibRecordsOut) Swap(i, j int) {
    records[i], records[j] = records[j], records[i]
}

func (in *BibRecordIn) Convert() *BibRecordOut {

	out := new(BibRecordOut)
   
    out.BibID = in.ID
    out.CreatedDate = in.CreatedDate

    for _, field := range in.Marc.Fields {
    	if field.Tag == "245" {
    		for _, subfield := range field.Data.Subfields {
    			out.TitleAndAuthor += subfield.Data
    		}
    	}
    	if field.Tag == "020" {
    		for _, subfield := range field.Data.Subfields {
    			if subfield.Code == "a"{
    				isbnField := strings.Split(subfield.Data, " ")
    				if len(isbnField) > 1 {
    					out.ISBNs = append(out.ISBNs, isbnField[0])
    				} else {
            	        out.ISBNs = append(out.ISBNs, subfield.Data)
    				}
    			}
    		}
    	}	
    }

	return out
}

func (in *BibRecordsIn) Convert() *BibRecordsOut {
	out := BibRecordsOut{}
	for _, bibRecord := range in.Entries {
		out = append(out, *bibRecord.Convert())
	}
	return &out
}

func SendRequestToAPI(apiURL, token string, w http.ResponseWriter, r *http.Request) (*http.Response, error) {

	l.Log(fmt.Sprintf("Sending request %v to Sierra API with token %v", apiURL, token), l.TraceMessage)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		http.Error(w, "Request failed.", http.StatusInternalServerError)
		return new(http.Response), err
	}

        req.Close = true

	err = SetAuthorizationHeaders(req, r, token)
	if err != nil {
		l.Log("The remote address in an incoming request is not set properly.", l.WarnMessage)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Error querying Sierra API.", http.StatusInternalServerError)
		return resp, err
	}
	l.Log(fmt.Sprintf("Sending response %#v back to caller", resp), l.TraceMessage)
	return resp, nil

}

//Set the required Authorization headers.
//This includes the Bearer token, User-Agent, and X-Forwarded-For
func SetAuthorizationHeaders(nr, or *http.Request, token string) error {
	nr.Header.Add("Authorization", "Bearer "+token)
	nr.Header.Add("User-Agent", "Tyro")

	originalForwardFor := or.Header.Get("X-Forwarded-For")
	if originalForwardFor == "" {
		ip, _, err := net.SplitHostPort(or.RemoteAddr)
		if err != nil {
			return err
		} else {
			nr.Header.Add("X-Forwarded-For", ip)
		}
	} else {
		nr.Header.Add("X-Forwarded-For", originalForwardFor)
	}

	return nil
}
