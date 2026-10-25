module github.com/dongsinhho/PhotoBooth

go 1.24.8

require (
	github.com/Kirizu-Official/windows-camera-go v0.2.0
	golang.org/x/image v0.32.0
)

require (
	golang.org/x/text v0.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Kirizu-Official/windows-camera-go => ./third_party/windows-camera-go
