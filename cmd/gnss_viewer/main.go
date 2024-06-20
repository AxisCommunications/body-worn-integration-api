package main

import (
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

//go:embed index.html
var content embed.FS

// Used to inject data to the index.html template
type PageData struct {
	GeoJson GeoJson
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error: Missing arguments, please provide a gps trail, -h for help")
		return
	}

	switch os.Args[1] {
	case "h", "-h", "--h", "help", "-help", "--help":
		printHelpMessage()
		return
	}

	geoJson, err := convertToGeoJson(os.Args[1])
	if err != nil {
		fmt.Printf("Error converting file: %v\n", err)
		return
	}

	data := PageData{GeoJson: geoJson}

	tmpl, err := template.ParseFS(content, "*")
	if err != nil {
		fmt.Println("Error parsing html template: ", err)
		return
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := tmpl.Execute(w, data)
		if err != nil {
			fmt.Println("Error executing template: ", err)
		}
	})

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		fmt.Println("Error listening to localhost: ", err)
		return
	}

	port := listener.Addr().(*net.TCPAddr).Port

	fmt.Println("Starting server at port ", port)
	go func() {
		if err := http.Serve(listener, nil); err != nil {
			fmt.Println("Error listening to localhost: ", err)
		}
	}()
	site := fmt.Sprintf("http://localhost:%v", port)
	err = openBrowser(site)
	if err != nil {
		fmt.Printf("Error opening browser, please go to %q to view the trail\n", site)
	}
	// ensure the server doesn't terminate.
	select {}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func printHelpMessage() {
	fmt.Println(`GNNS viewer example usage

Arguments
  -help		Show this message.
  <filepath>	Filepath to a GNNS trail file that should be viewed.

<GNNS trail file>.json
  This file should be formated according to Open API standard GNNS format.`)

}
