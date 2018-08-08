package controller

import (
	"encoding/xml"
	"fmt"
	"github.com/brave/go-update/extension"
	"github.com/go-chi/chi"
	"github.com/pressly/lg"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var allExtensions extension.Extensions

// ExtensionsRouter is the router for /extensions endpoints
func ExtensionsRouter(extensions extension.Extensions) chi.Router {
	allExtensions = extensions
	r := chi.NewRouter()
	r.Post("/", UpdateExtensions)
	r.Get("/", WebStoreUpdateExtension)
	return r
}

// WebStoreUpdateExtension is the handler for updating a single extension made via the GET HTTP methhod.
// Get requests look like this:
// /extensions?os=mac&arch=x64&os_arch=x86_64&nacl_arch=x86-64&prod=chromiumcrx&prodchannel=&prodversion=69.0.54.0&lang=en-US&acceptformat=crx2,crx3&x=id%3Doemmndcbldboiebfnladdacbdfmadadm%26v%3D0.0.0.0%26installedby%3Dpolicy%26uc%26ping%3Dr%253D-1%2526e%253D1"
// The query parameter x contains the encoded extension information.
func WebStoreUpdateExtension(w http.ResponseWriter, r *http.Request) {
	log := lg.Log(r.Context())
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Errorf("Error closing body stream: %v", err)
		}
	}()

	x := r.FormValue("x")
	unescaped, err := url.QueryUnescape(x)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unescaping query parameters: %v", err), http.StatusBadRequest)
		return
	}
	values, err := url.ParseQuery(unescaped)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing query parameters: %v", err), http.StatusBadRequest)
		return
	}

	id := strings.Trim(values.Get("id"), "[]")
	v := values.Get("v")
	if len(id) == 0 {
		http.Error(w, fmt.Sprintf("No extension ID specified."), http.StatusBadRequest)
		return
	}

	webStoreResponse := extension.WebStoreUpdateResponse{}
	foundExtension, err := allExtensions.Contains(id)
	if err != nil {
		http.Redirect(w, r, "https://clients2.google.com/service/update2/crx?"+r.URL.RawQuery+"&braveRedirect=true", http.StatusTemporaryRedirect)
		return
	}

	if extension.CompareVersions(v, foundExtension.Version) < 0 {
		webStoreResponse.ID = foundExtension.ID
		webStoreResponse.Version = foundExtension.Version
		webStoreResponse.SHA256 = foundExtension.SHA256
	}

	w.Header().Set("content-type", "application/xml")
	w.WriteHeader(http.StatusOK)
	data, err := xml.Marshal(&webStoreResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error in marshal XML %v", err), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		log.Errorf("Error writing response: %v", err)
	}
}

// UpdateExtensions is the handler for updating extensions
func UpdateExtensions(w http.ResponseWriter, r *http.Request) {
	log := lg.Log(r.Context())
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Errorf("Error closing body stream: %v", err)
		}
	}()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading body: %v", err), http.StatusBadRequest)
		return
	}

	updateRequest := extension.UpdateRequest{}
	err = xml.Unmarshal(body, &updateRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading body %v", err), http.StatusBadRequest)
		return
	}
	// Special case, if there's only 1 extension in the request and it is not something
	// we know about, redirect the client to google component update server.
	if len(updateRequest) == 1 {
		_, err := allExtensions.Contains(updateRequest[0].ID)
		if err != nil {
			http.Redirect(w, r, "https://update.googleapis.com/service/update2?braveRedirect=true", http.StatusTemporaryRedirect)
			return
		}
	}
	w.Header().Set("content-type", "application/xml")
	w.WriteHeader(http.StatusOK)
	updateResponse := allExtensions.FilterForUpdates(updateRequest)
	data, err := xml.Marshal(&updateResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error in marshal XML %v", err), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		log.Errorf("Error writing response: %v", err)
	}
}
