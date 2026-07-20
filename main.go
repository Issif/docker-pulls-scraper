package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	yaml "gopkg.in/yaml.v3"
)

type List struct {
	Images []string `yaml:"images"`
	Sums   []struct {
		Name   string   `yaml:"name"`
		Images []string `yaml:"images"`
	} `yaml:"sums"`
	Versions []struct {
		Images   []string          `yaml:"images"`
		Releases map[string]string `yaml:"releases"`
	} `yaml:"versions"`
}

type Image struct {
	Name       string
	Count      int `json:"pull_count"`
	HumanCount string
}

var (
	list   List
	images []Image
)

const (
	URLBase = "https://hub.docker.com/v2/repositories/"
)

func init() {
	images = make([]Image, 0)
}

func main() {
	l := flag.String("l", "", "YAML file with the list of images to track")
	dataFolder := flag.String("d", "./data", "Destination folder for .csv")
	flag.Parse()

	if *l == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := os.Mkdir(*dataFolder, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
		log.Printf("Folder '%v' already exists\n", *dataFolder)
	}

	yamlFile, err := os.ReadFile(*l)
	if err != nil {
		log.Fatalf("Read file: '%v'\n", err)
	}
	if err := yaml.Unmarshal(yamlFile, &list); err != nil {
		log.Fatalf("Unmarshal: '%v'\n", err)
	}

	for _, i := range list.Images {
		getPullCount(i)
	}

	for _, i := range list.Sums {
		s := Image{
			Name: "SUM/" + i.Name,
		}
		for _, j := range i.Images {
			for _, k := range images {
				if j == k.Name {
					s.Count += k.Count
				}
			}
		}
		s.HumanCount = humanize.Comma(int64(s.Count))
		images = append(images, s)
	}

	for _, i := range images {
		writeCSV(i, *dataFolder)
	}

	updateIndexHTML()
}

func writeCSV(image Image, dataFolder string) {
	filename := fmt.Sprintf("%v/%v.csv", dataFolder, strings.ReplaceAll(image.Name, "/", "_"))
	log.Printf("Writing of the .csv for the image '%v' in '%v'\n", image.Name, filename)

	_, errorExist := os.Stat(filename)
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var delta int

	if os.IsNotExist(errorExist) {
		if _, err := f.WriteString("Date,Count,Delta\n"); err != nil {
			log.Fatal(err)
		}
	} else {
		var lastLine string
		fileScanner := bufio.NewScanner(f)
		for fileScanner.Scan() {
			if l := fileScanner.Text(); l != "" {
				lastLine = l
			}
		}
		if err := fileScanner.Err(); err != nil {
			log.Fatal(err)
		}
		lastCount, _ := strconv.Atoi(strings.Split(lastLine, ",")[1])
		delta = image.Count - lastCount
	}

	if _, err := f.WriteString(fmt.Sprintf("%v,%v,%v\n", time.Now().Format("2006/01/02"), image.Count, delta)); err != nil {
		log.Fatal(err)
	}
}

type ManifestEntry struct {
	Name       string            `json:"name"`
	File       string            `json:"file"`
	Count      int               `json:"count"`
	HumanCount string            `json:"humanCount"`
	IsSum      bool              `json:"isSum"`
	Versions   map[string]string `json:"versions,omitempty"`
}

type IndexData struct {
	Images       []Image
	ManifestJSON template.JS
}

func versionsForImage(name string) map[string]string {
	releases := make(map[string]string)
	for _, i := range list.Versions {
		for _, j := range i.Images {
			if name == j {
				maps.Copy(releases, i.Releases)
			}
		}
	}
	if len(releases) == 0 {
		return nil
	}
	return releases
}

func updateIndexHTML() {
	log.Println("Writing of the index.html")

	sort.Slice(images, func(i, j int) bool {
		return images[i].Count > images[j].Count
	})

	manifest := make([]ManifestEntry, 0, len(images))
	for _, i := range images {
		manifest = append(manifest, ManifestEntry{
			Name:       i.Name,
			File:       strings.ReplaceAll(i.Name, "/", "_") + ".csv",
			Count:      i.Count,
			HumanCount: i.HumanCount,
			IsSum:      strings.HasPrefix(i.Name, "SUM"),
			Versions:   versionsForImage(i.Name),
		})
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		log.Fatal(err)
	}

	templateStr := `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css">
	<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@24,400,0,0" />
	<script src="https://go-echarts.github.io/go-echarts-assets/assets/echarts.min.js"></script>
	<title>Docker pull counts</title>
	<style>
		tr[data-name] { cursor: pointer; }
		tr[data-name].active-chart { background-color: #e3f2fd; }
		.stats-row { display: flex; gap: 16px; margin: 20px 20px 0 20px; }
		.stat-card { flex: 1; background: #f5f5f5; border-radius: 6px; padding: 12px 16px; box-shadow: 0 1px 3px rgba(0,0,0,.15); }
		.stat-label { font-size: .8rem; color: #666; text-transform: uppercase; letter-spacing: .03em; }
		.stat-value { font-size: 1.4rem; font-weight: 600; margin-top: 4px; }
		.stat-value.up { color: #2e7d32; }
		.stat-value.down { color: #c62828; }
		#chart-title { margin: 20px 0 0 20px; font-size: 1.5rem; font-weight: 500; }
	</style>
</head>
<body>
	<div class="row flex">
		<div class="col s3">
		<table class="striped responsive-table" style="margin: 20px">
			<caption>SUMS</caption>
			<thead>
				<tr>
					<th>Image</th>
					<th>Last count</th>
					<th>Chart</th>
				</tr>
			</thead>
			<tbody>
				{{- range .Images }}
				{{ if (hasPrefix .Name "SUM") }}
					<tr data-name="{{ .Name }}">
						<td><a href="https://hub.docker.com/r/{{ .Name }}" target="_blank" rel="noopener">{{ .Name }}</a></td>
						<td>{{ .HumanCount }}</td>
						<td><span class="material-symbols-outlined">monitoring</span></td>
					</tr>
				{{ end }}
				{{- end }}
			</tbody>
		</table>
		<table class="striped responsive-table" style="margin: 20px">
			<caption>IMAGES</caption>
			<thead>
				<tr>
					<th>Image</th>
					<th>Last count</th>
					<th>Chart</th>
				</tr>
			</thead>
			<tbody>
				{{- range .Images }}
				{{ if not (hasPrefix .Name "SUM") }}
					<tr data-name="{{ .Name }}">
						<td><a href="https://hub.docker.com/r/{{ .Name }}" target="_blank" rel="noopener">{{ .Name }}</a></td>
						<td>{{ .HumanCount }}</td>
						<td><span class="material-symbols-outlined">monitoring</span></td>
					</tr>
				{{ end }}
				{{- end }}
			</tbody>
		</table>
		</div>
		<div class="col s9">
			<div id="chart-title"></div>
			<div class="stats-row">
				<div class="stat-card">
					<div class="stat-label">Total</div>
					<div class="stat-value" id="stat-total">-</div>
				</div>
				<div class="stat-card">
					<div class="stat-label">24h change</div>
					<div class="stat-value" id="stat-1d">-</div>
				</div>
				<div class="stat-card">
					<div class="stat-label">7d change</div>
					<div class="stat-value" id="stat-7d">-</div>
				</div>
				<div class="stat-card">
					<div class="stat-label">30d change</div>
					<div class="stat-value" id="stat-30d">-</div>
				</div>
			</div>
			<div id="chart" style="width: 100%; height: 75vh; margin-top: 10px;"></div>
		</div>
	</div>
	<script>
		"use strict";
		const MANIFEST = {{ .ManifestJSON }};
		const manifestByName = {};
		MANIFEST.forEach(function (m) { manifestByName[m.name] = m; });

		const chartEl = document.getElementById("chart");
		const chartTitleEl = document.getElementById("chart-title");
		let chartInstance = null;
		const csvCache = {};

		function parseCSV(text) {
			const lines = text.trim().split("\n");
			lines.shift();
			return lines.filter(Boolean).map(function (line) {
				const parts = line.split(",");
				return { date: parts[0], count: parseInt(parts[1], 10), delta: parseInt(parts[2], 10) };
			});
		}

		function pctChange(rows, daysBack) {
			const n = rows.length;
			const idx = n - 1 - daysBack;
			if (idx < 0) return null;
			const past = rows[idx].count;
			const now = rows[n - 1].count;
			if (!past) return null;
			return ((now - past) / past) * 100;
		}

		function formatPct(value) {
			if (value === null) return "N/A";
			const sign = value > 0 ? "+" : "";
			return sign + value.toFixed(2) + "%";
		}

		function setStat(id, value) {
			const el = document.getElementById(id);
			el.textContent = formatPct(value);
			el.classList.remove("up", "down");
			if (value !== null) {
				el.classList.add(value >= 0 ? "up" : "down");
			}
		}

		function renderStats(rows) {
			const total = rows[rows.length - 1].count;
			document.getElementById("stat-total").textContent = total.toLocaleString("en-US");
			setStat("stat-1d", pctChange(rows, 1));
			setStat("stat-7d", pctChange(rows, 7));
			setStat("stat-30d", pctChange(rows, 30));
		}

		function renderImageChart(entry, rows) {
			if (!chartInstance) {
				chartInstance = echarts.init(chartEl, "white", { renderer: "canvas" });
			}

			const markLineData = [];
			if (entry.versions) {
				Object.keys(entry.versions).forEach(function (date) {
					markLineData.push({ name: entry.versions[date], xAxis: date });
				});
			}

			const option = {
				color: ["blue", "orange"],
				dataZoom: [{ type: "slider", start: 0, end: 100 }],
				legend: { show: true, selectedMode: "multiple" },
				tooltip: {
					show: true,
					trigger: "axis",
					axisPointer: { type: "cross", snap: true }
				},
				xAxis: { type: "category", data: rows.map(function (r) { return r.date; }) },
				yAxis: [
					{ name: "# pulls", type: "value", show: true },
					{ name: "delta", type: "value", show: true, scale: true }
				],
				series: [
					{
						name: "# pulls",
						type: "line",
						showSymbol: true,
						data: rows.map(function (r) { return r.count; }),
						markLine: markLineData.length ? {
							symbol: "none",
							label: { show: true, formatter: "{b}" },
							lineStyle: { color: "gray" },
							data: markLineData
						} : undefined
					},
					{
						name: "delta",
						type: "line",
						showSymbol: true,
						yAxisIndex: 1,
						data: rows.map(function (r) { return r.delta; })
					}
				]
			};

			chartInstance.setOption(option, true);
		}

		async function selectImage(name, updateHash) {
			const entry = manifestByName[name];
			if (!entry) return;

			document.querySelectorAll("tr[data-name]").forEach(function (tr) {
				tr.classList.toggle("active-chart", tr.getAttribute("data-name") === name);
			});
			chartTitleEl.textContent = entry.name;

			if (updateHash !== false) {
				history.replaceState(null, "", "#" + encodeURIComponent(name));
			}

			let rows = csvCache[entry.file];
			if (!rows) {
				const res = await fetch("data/" + entry.file);
				const text = await res.text();
				rows = parseCSV(text);
				csvCache[entry.file] = rows;
			}

			renderStats(rows);
			renderImageChart(entry, rows);
		}

		document.querySelectorAll("tbody").forEach(function (tbody) {
			tbody.addEventListener("click", function (e) {
				const tr = e.target.closest("tr[data-name]");
				if (!tr) return;
				selectImage(tr.getAttribute("data-name"));
			});
		});

		window.addEventListener("resize", function () {
			if (chartInstance) chartInstance.resize();
		});

		window.addEventListener("hashchange", function () {
			const name = decodeURIComponent(location.hash.slice(1));
			if (manifestByName[name]) selectImage(name);
		});

		if (MANIFEST.length) {
			const fromHash = decodeURIComponent(location.hash.slice(1));
			selectImage(manifestByName[fromHash] ? fromHash : MANIFEST[0].name);
		}
	</script>
</body>
</html>`
	parsedTemplate, err := template.
		New("index").
		Funcs(template.FuncMap{
			"hasPrefix": strings.HasPrefix,
		}).
		Parse(templateStr)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile("index.html", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0744)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	data := IndexData{
		Images:       images,
		ManifestJSON: template.JS(manifestJSON),
	}

	err = parsedTemplate.Execute(f, data)
	if err != nil {
		log.Fatal(err)
	}
}

func getPullCount(image string) {
	log.Printf("Scrape pull count for the image '%v'\n", image)
	res, err := http.Get(URLBase + image)
	if err != nil {
		log.Fatal(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var i Image
	json.Unmarshal(body, &i)
	i.Name = image
	i.HumanCount = humanize.Comma(int64(i.Count))
	images = append(images, i)
}

func falcosidekick_versions() map[string]string {
	releases := map[string]string{
		"2018/10/10": "1.0.1",
		"2018/10/13": "1.0.2",
		"2019/01/30": "1.0.3",
		"2019/02/01": "1.0.4",
		"2019/04/09": "1.0.5",
		"2019/05/09": "1.0.7",
		"2019/05/10": "1.1.0",
		"2019/05/23": "2.0.0",
		"2019/06/12": "2.1.0",
		"2019/06/13": "2.2.0",
		"2019/06/17": "2.3.0",
		"2019/06/26": "2.4.0",
		"2019/08/12": "2.5.0",
		"2019/08/26": "2.6.0",
		"2019/08/27": "2.7.0",
		"2019/08/28": "2.7.2",
		"2019/09/11": "2.8.0",
		"2019/10/04": "2.9.0",
		"2019/10/07": "2.9.1",
		"2019/10/11": "2.9.2",
		"2019/10/18": "2.9.3",
		"2019/10/22": "2.10.0",
		"2019/11/13": "2.11.0",
		"2020/01/06": "2.11.1",
		"2020/01/16": "2.12.0",
		"2020/01/28": "2.12.1",
		"2020/04/21": "2.12.3",
		"2020/06/14": "2.13.0",
		"2020/08/08": "2.14.0",
		"2020/10/28": "2.15.0",
		"2020/10/29": "2.16.0",
		"2020/11/15": "2.17.0",
		"2020/11/20": "2.18.0",
		"2020/12/01": "2.19.0",
		"2020/12/02": "2.19.1",
		"2021/01/13": "2.20.0",
		"2021/02/12": "2.21.0",
		"2021/04/06": "2.22.0",
		"2021/06/22": "2.23.0",
		"2021/06/23": "2.23.1",
		"2021/08/16": "2.24.0",
		"2022/05/12": "2.25.0",
		"2022/06/17": "2.26.0",
		"2023/01/09": "2.27.0",
		"2023/07/27": "2.28.0",
		"2024/07/02": "2.29.0",
		"2024/11/27": "2.30.0",
		"2025/02/03": "2.31.0",
		"2025/02/04": "2.31.1",
	}
	return releases
}

func falco_versions() map[string]string {
	releases := map[string]string{
		"2023/06/07": "0.35.0",
		"2023/06/29": "0.35.1",
		"2023/09/26": "0.36.0",
		"2023/10/16": "0.36.1",
		"2023/10/27": "0.36.2",
		"2024/01/30": "0.37.0",
		"2024/02/13": "0.37.1",
		"2024/05/30": "0.38.0",
		"2024/06/19": "0.38.1",
		"2024/08/19": "0.38.2",
		"2024/10/01": "0.39.0",
		"2024/10/09": "0.39.1",
		"2024/11/21": "0.39.2",
		"2025/01/28": "0.40.0",
	}
	return releases
}

func falcoctl_versions() map[string]string {
	releases := map[string]string{
		"2019/09/04": "0.0.1",
		"2019/10/14": "0.0.2",
		"2019/10/22": "0.0.3",
		"2019/10/24": "0.0.4",
		"2019/10/28": "0.0.5",
		"2019/10/31": "0.0.6",
		"2019/11/07": "0.0.7",
		"2020/07/15": "0.1.0",
		"2023/02/02": "0.3.0",
		"2023/02/06": "0.4.0",
		"2023/05/23": "0.5.0",
		"2023/06/12": "0.5.1",
		"2023/09/04": "0.6.0",
		"2023/09/18": "0.6.1",
		"2023/09/22": "0.6.2",
		"2024/01/12": "0.7.0",
		"2024/01/23": "0.7.1",
		"2024/02/12": "0.7.2",
		"2024/02/22": "0.7.3",
		"2024/05/28": "0.8.0",
		"2024/08/01": "0.9.0",
		"2024/09/03": "0.9.1",
		"2024/09/16": "0.10.0",
		"2024/11/21": "0.10.1",
		"2025/01/27": "0.11.0",
	}
	return releases
}

func falcosidekick_ui_versions() map[string]string {
	releases := map[string]string{
		"2021/02/11": "0.1.0",
		"2021/02/26": "0.2.0",
		"2021/06/23": "1.1.0",
		"2022/05/11": "2.0.0",
		"2022/05/25": "2.0.1",
		"2022/06/05": "2.0.2",
		"2023/01/10": "2.1.0",
		"2023/09/14": "2.2.0",
	}
	return releases
}

func falco_talon_versions() map[string]string {
	releases := map[string]string{
		"2024/09/06": "0.1.0",
		"2024/10/01": "0.1.1",
		"2024/11/27": "0.2.0",
		"2024/12/09": "0.2.1",
		"2025/02/07": "0.3.0",
	}
	return releases
}
