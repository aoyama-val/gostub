package main

import (
	"fmt"
	"log"
	"net/http"
	"io/ioutil"
	"os"
	"regexp"
	"encoding/json"
	"strings"
	"errors"
	"github.com/gostub/gostub/models"
)

type Gostub struct {
	port string
	outputPath string
}

func (g *Gostub) Run() {
	http.HandleFunc("/", g.HandleStubRequest)
	http.HandleFunc("/gostub/shutdown", handleShutdown)
	portAddress := ":" + g.port
	log.Fatal(http.ListenAndServe(portAddress, nil))
}

func (g *Gostub) HandleStubRequest(w http.ResponseWriter, r *http.Request) {
	pathPatternList := g.RecursiveGetFilePath(r.Method)
	requestPath := r.URL.Path
	result, matchError := g.MatchRoute(pathPatternList, requestPath)
	if matchError != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Not found path content (%v)", requestPath)
		return
	}
	matchPattern := *result
	contentPath := matchPattern + "/$" + strings.ToUpper(r.Method) + ".json"
	content, readError := ioutil.ReadFile("." + contentPath)
	if readError != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid path content (%v)", contentPath)
		return
	}
	list := new(models.ContentList)
	json.Unmarshal(content, &list)
	w.Header().Set("Content-Type", "application/json")
	for _, handler := range list.Handlers {
		if isMatchRequest(r, handler) {
			w.WriteHeader(handler.Content.Status)
			bodyFilePath := matchPattern + "/" + handler.Content.Body
			if strings.HasPrefix(handler.Content.Body, "/") {
				bodyFilePath = "/" + g.outputPath + handler.Content.Body
			}
			response, _ := ioutil.ReadFile("." + bodyFilePath)
			fmt.Fprint(w, string(response))
			return
		}
	}

	// TODO: header、cookieも付けられるように改良
	w.WriteHeader(list.Default.Status)
	bodyFilePath := matchPattern + "/" + list.Default.Body
	if strings.HasPrefix(list.Default.Body, "/") {
		bodyFilePath = "/" + g.outputPath + list.Default.Body
	}
	response, _ := ioutil.ReadFile("." + bodyFilePath)
	fmt.Printf("bodyFilePath: %v\n", bodyFilePath)
	fmt.Fprint(w, string(response))
}

func (g *Gostub) RecursiveGetFilePath(method string) []string {
	var pathPatternList []string
	g.recursiveGetFilePath(g.RootPath(), method, &pathPatternList)
	return pathPatternList
}

func (g *Gostub) recursiveGetFilePath(path string, method string, pathList *[]string) {
	files, _ := ioutil.ReadDir("." + path)
	for _, f := range files {
		if f.IsDir() {
			subPath := path + f.Name() + "/"
			if exists(subPath + "$" + method + ".json") {
				*pathList = append(*pathList, path + f.Name())
			}
			g.recursiveGetFilePath(subPath, method, pathList)
		}
	}
}

func (g *Gostub) RootPath() string {
	if g.outputPath == "" {
		return "/"
	}
	return fmt.Sprintf("/%v/", g.outputPath)
}

func (g *Gostub) MatchRoute(pathList []string, requestPath string) (*string, error) {
	if g.outputPath != "" {
		requestPath = "/" + g.outputPath + requestPath
	}
	filteredPathPatternList := filtered(pathList, func(p string) bool {
		return isMatchRegex("^" +  p + "$", requestPath)
	})
	if len(filteredPathPatternList) == 0 {
		return nil, errors.New("not found route")
	}
	// FIXME とりあえず一番最後のpathを指定
	n := len(filteredPathPatternList)
	return &filteredPathPatternList[n-1], nil
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Fatal("Stop gostub server.")
}

func isMatchRequest(r *http.Request, handler models.Handler) bool {
	for k ,v := range handler.Header {
		if !isMatchRegex(v, r.Header.Get(k)) {
			return false
		}
	}
	for k ,v := range handler.Param {
		if r.Method == http.MethodGet {
			if !isMatchRegex(v, r.URL.Query().Get(k)) {
				return false
			}
		} else if r.Method == http.MethodPost {
			if !isMatchRegex(v, r.PostForm.Get(k)) {
				return false
			}
		}
	}
	return true
}

func filtered(strings []string, filter func(string,) bool) []string {
	var res []string
	for _, path := range strings {
		if filter(path) {
			res = append(res, path)
		}
	}
	return res
}

func exists(filename string) bool {
	_, err := os.Stat("." + filename)
	return err == nil
}

func isMatchRegex(regexPattern string, target string) bool {
	regex := regexp.MustCompile(regexPattern)
	return regex.MatchString(target)
}
