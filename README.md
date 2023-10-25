# Docker pulls scraper

DockerHub provides an API endpoint to track the number of pulls of the images it hosts but no history. This small program allows to get that count and updates a .csv file to keep a record.
It generates one `.csv` file per image, with the columns: `Date, Count`. Up to you to run the script every X times to add a new line in the files.

## Build

```shell
go build
```

## Usage

```shell
Usage of docker-pulls-scraper:
  -d string
        Destination folder for .csv (default "./data")
  -l string
        File with the list of images to track
  -r string
        Destination folder for the .html (default "./render")
```

## List of images

The list of images must be in file, with one image per line, like this:

```
owner/image-1
owner/image-2
owner/image-3
```

### Results

Log:
```
❯ go run . -l list.txt
2023/10/17 18:34:58 Folder './data' already exists
2023/10/17 18:34:58 Scrape pull count for the image 'falcosecurity/falco'
2023/10/17 18:34:59 Writing of the .csv for the image 'falcosecurity/falco' in './data/falcosecurity_falco.csv'
2023/10/17 18:34:59 Scrape pull count for the image 'falcosecurity/falcosidekick'
2023/10/17 18:34:59 Writing of the .csv for the image 'falcosecurity/falcosidekick' in './data/falcosecurity_falcosidekick.csv'
2023/10/17 18:34:59 Scrape pull count for the image 'falcosecurity/falcosidekick-ui'
2023/10/17 18:34:59 Writing of the .csv for the image 'falcosecurity/falcosidekick-ui' in './data/falcosecurity_falcosidekick-ui.csv'
2023/10/17 18:34:59 Scrape pull count for the image 'falcosecurity/driverkit'
2023/10/17 18:34:59 Writing of the .csv for the image 'falcosecurity/driverkit' in './data/falcosecurity_driverkit.csv'
2023/10/17 18:34:59 Scrape pull count for the image 'falcosecurity/falcoctl'
2023/10/17 18:34:59 Writing of the .csv for the image 'falcosecurity/falcoctl' in './data/falcosecurity_falcoctl.csv'
```

Example of a `.csv` file:
```csv
Date,Count
2023/03/15,43254957
2023/03/16,43268631
2023/03/17,43283486
2023/03/18,43293650
```

## Author

Thomas Labarussias (https://github.com/Issif)
