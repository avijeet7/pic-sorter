package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// Extract GPS coordinates from an image
func getGeoInfo(imagePath string) (float64, float64, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return 0, 0, err
	}

	lat, lon, err := x.LatLong()
	if err != nil {
		return 0, 0, err
	}

	return lat, lon, nil
}

// Fetch location details (country, state, state district, county)
func getLocationDetails(lat, lon float64) (map[string]string, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f&zoom=10", lat, lon)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	address, ok := data["address"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid address data")
	}

	location := map[string]string{
		"country":        getString(address, "country"),
		"state":          getString(address, "state"),
		"state_district": getString(address, "state_district"),
		"county":         getString(address, "county"),
	}

	return location, nil
}

// Helper function to get string from map
func getString(data map[string]interface{}, key string) string {
	if value, found := data[key]; found {
		return fmt.Sprintf("%v", value)
	}
	return "Unknown"
}

// Move image to the correct folder based on location
func moveImage(imagePath string, location map[string]string) error {
	// Create folder structure: country/state/state_district/county/
	folderPath := filepath.Join(
		"sorted_images",
		sanitize(location["country"]),
		sanitize(location["state"]),
		sanitize(location["state_district"]),
		sanitize(location["county"]),
	)

	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return err
	}

	newImagePath := filepath.Join(folderPath, filepath.Base(imagePath))
	return os.Rename(imagePath, newImagePath)
}

// Sanitize folder names to remove special characters
func sanitize(name string) string {
	return strings.ReplaceAll(name, " ", "_")
}

// Process all images in a directory
func processImages(directory string) {
	files, err := os.ReadDir(directory)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".jpg") || strings.HasSuffix(file.Name(), ".jpeg") || strings.HasSuffix(file.Name(), ".png")) {
			imagePath := filepath.Join(directory, file.Name())

			lat, lon, err := getGeoInfo(imagePath)
			if err != nil {
				fmt.Printf("No GPS data found for %s\n", file.Name())
				continue
			}

			location, err := getLocationDetails(lat, lon)
			if err != nil {
				fmt.Printf("Error getting location for %s: %s\n", file.Name(), err)
				continue
			}

			fmt.Printf("Moving %s to %s/%s/%s/%s\n",
				file.Name(),
				location["country"], location["state"], location["state_district"], location["county"],
			)

			if err := moveImage(imagePath, location); err != nil {
				fmt.Printf("Error moving file: %s\n", err)
			}
		}
	}
}

func main() {
	imageDirectory := "images" // Change this to your folder containing images
	processImages(imageDirectory)
}
