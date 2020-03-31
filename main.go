package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/fatih/color"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	_ "github.com/guptarohit/asciigraph"
	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

const URI = "https://pomber.github.io/covid19/timeseries.json"

type DayItem struct {
	Date      string `json:"date"`
	Confirmed int    `json:"confirmed"`
	Deaths    int    `json:"deaths"`
	Recovered int    `json:"recovered"`
}

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(i, j int) bool { return p[i].Value > p[j].Value }

var AllValues = make(map[string][]DayItem)
var NameSortedCountries = make([]string, 0)
var ValueSortedCountries = make(PairList, 0)
var Verbose bool

func main() {
	generalFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:        "verbose",
			Aliases:     []string{"v"},
			Usage:       "Use more verbosity",
			Destination: &Verbose,
		},
	}
	app := &cli.App{
		Name:  "covid19",
		Usage: "Track the COVID-19 (2019 novel Coronavirus) in the command line. ",
		Action: func(c *cli.Context) error {
			if err := PrintSummary(); err != nil {
				color.Red("Error Parsing Data, Details: %v", err)
			}
			return nil
		},
		Flags: generalFlags,
		Commands: []*cli.Command{
			{
				Name:    "fetch",
				Aliases: []string{"f"},
				Usage:   "Fetch Data to Temp for further analysis, avoid fetch from web ",
				Action: func(c *cli.Context) error {
					if err := FetchData(true); err != nil {
						color.Red("Error Fetching Data, Details: %v", err)
					}
					return nil
				},
			},
			{
				Name:    "summary",
				Aliases: []string{"s"},
				Usage:   "Show country summary of cases, deaths and recoveries",
				Action: func(c *cli.Context) error {
					country := c.String("country")
					// prompt := promptui.Prompt{
					// 	Label:     "Country Name [all]",
					// }
					// inputCountry, _ := prompt.Run()
					// if inputCountry != "" {
					// 	country = inputCountry
					// }
					if err := PrintCountry(country, c.Int("max")); err != nil {
						color.Red("Error Parsing Data, Details: %v", err)
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max",
						Aliases: []string{"m"},
						Usage:   "Maximum number of countries to be printed for [all]",
						Value:   100,
					},
					&cli.StringFlag{
						Name:    "country",
						Aliases: []string{"c"},
						Usage:   "Specify the country name to be used for summary",
						Value:   "all",
					},
				},
			},
			{
				Name:    "chart",
				Aliases: []string{"c"},
				Usage:   "Draw chart of cases, deaths and recoveries for each countries",
				Action: func(c *cli.Context) error {
					country := c.String("country")
					drawType := c.String("type")
					if drawType == "bar" {
						if err := DrawCountryBarChart(country, c.Int("max")); err != nil {
							color.Red("Error Parsing Data, Details: %v", err)
						}
					}else{
						if err := DrawCountryLineChart(country, c.Int("max")); err != nil {
							color.Red("Error Parsing Data, Details: %v", err)
						}
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max",
						Aliases: []string{"m"},
						Usage:   "Maximum number of days to be printed form last day",
						Value:   10,
					},
					&cli.StringFlag{
						Name:    "country",
						Aliases: []string{"c"},
						Usage:   "Specify the country name to be used for draw chart",
						Value:   "all",
					},
					&cli.StringFlag{
						Name:    "type",
						Aliases: []string{"t","x"},
						Usage:   "Specify chart type [bar, line]",
						Value:   "bar",
					},
				},
			},
		},
	}
	
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	// data := []float64{3, 4, 9, 6, 2, 4, 5, 8, 5, 10, 2, 7, 2, 5, 6}
	// graph := asciigraph.Plot(data, asciigraph.Width(50))
	//
	// fmt.Println(graph)
}

func FetchData(saveToTempOnly bool) error {
	if Verbose {
		color.Cyan("Fetching Data ...")
	}
	
	client := http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	request, err := http.NewRequest("GET", URI, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	
	arrayData, _ := ioutil.ReadAll(response.Body)
	
	if saveToTempOnly {
		filename := fmt.Sprintf("%s%s", os.TempDir(), "covid_19_timeseries.json")
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		_, err = f.Write(arrayData)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
		color.Blue("File Fetched in path: %s", filename)
		return nil
	}
	
	if Verbose {
		color.Blue("Parsing Data ...")
	}
	err = jsonparser.ObjectEach(arrayData, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		country := string(key)
		countryItems := make([]DayItem, 0)
		jsonparser.ArrayEach(value, func(arrValue []byte, dataType jsonparser.ValueType, offset int, err error) {
			var dayItem DayItem
			if err := json.Unmarshal(arrValue, &dayItem); err != nil {
				log.Print(err)
			}
			countryItems = append(countryItems, dayItem)
		})
		AllValues[country] = countryItems
		return nil
	}, )
	for k := range AllValues {
		NameSortedCountries = append(NameSortedCountries, k)
	}
	sort.Strings(NameSortedCountries)
	
	for c, v := range AllValues {
		ValueSortedCountries = append(ValueSortedCountries, Pair{c, v[len(v)-1].Confirmed})
	}
	sort.Sort(ValueSortedCountries)
	return err
}

func PrintSummary() error {
	if err := FetchData(false); err != nil {
		return err
	}
	Confirmed, Deaths, Recovered := 0, 0, 0
	for _, items := range AllValues {
		lItem := items[len(items)-1]
		Confirmed += lItem.Confirmed
		Deaths += lItem.Deaths
		Recovered += lItem.Recovered
	}
	
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Confirmed", "Deaths", "Recovered"})
	t.AppendRows([]table.Row{
		{Confirmed, Deaths, Recovered},
	})
	t.SetStyle(table.StyleColoredRedWhiteOnBlack)
	t.Render()
	return nil
}

func PrintCountry(country string, max int) error {
	if err := FetchData(false); err != nil {
		return err
	}
	// TODO: Add Last Update Date
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Country", "Confirmed", "Deaths", "Recovered", "New Case", "New Deaths", "New Recoveries"})
	
	if country == "all" {
		for idx, Pair := range ValueSortedCountries {
			if idx == max {
				break
			}
			lItem := AllValues[Pair.Key][len(AllValues[Pair.Key])-1]
			mlItem := AllValues[Pair.Key][len(AllValues[Pair.Key])-2]
			newConfirmed := lItem.Confirmed - mlItem.Confirmed
			newDeaths := lItem.Deaths - mlItem.Deaths
			newRecovered := lItem.Recovered - mlItem.Recovered
			t.AppendRows([]table.Row{
				{Pair.Key, lItem.Confirmed, lItem.Deaths, lItem.Recovered, newConfirmed, newDeaths, newRecovered},
			})
		}
	} else {
		if countryItem, ok := AllValues[country]; ok {
			lItem := countryItem[len(countryItem)-1]
			mlItem := countryItem[len(countryItem)-2]
			newConfirmed := lItem.Confirmed - mlItem.Confirmed
			newDeaths := lItem.Deaths - mlItem.Deaths
			newRecovered := lItem.Recovered - mlItem.Recovered
			t.AppendRows([]table.Row{
				{country, lItem.Confirmed, lItem.Deaths, lItem.Recovered, newConfirmed, newDeaths, newRecovered},
			})
		} else {
			return errors.New("country not found in dataset")
		}
	}
	t.SetStyle(table.StyleColoredRedWhiteOnBlack)
	t.Render()
	return nil
}

func DrawCountryBarChart(country string, max int) error {
	if err := FetchData(false); err != nil {
		return err
	}
	
	cChart := widgets.NewBarChart()
	cChart.Title = fmt.Sprintf("Confirmed Case Chart [%s]", country)
	
	dChart := widgets.NewBarChart()
	dChart.Title = fmt.Sprintf("Deaths Chart [%s]", country)
	
	rChart := widgets.NewBarChart()
	rChart.Title = fmt.Sprintf("Recovered Chart [%s]", country)
	
	cData, dData,rData := make([]float64,0),make([]float64,0),make([]float64,0)
	
	cChart.BarColors = []ui.Color{ui.ColorYellow, }
	dChart.BarColors = []ui.Color{ui.ColorMagenta}
	rChart.BarColors = []ui.Color{ui.ColorGreen}
	
	cChart.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
	dChart.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
	rChart.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
	
	cChart.NumStyles = []ui.Style{ui.NewStyle(ui.ColorBlack)}
	dChart.NumStyles = []ui.Style{ui.NewStyle(ui.ColorBlack)}
	rChart.NumStyles = []ui.Style{ui.NewStyle(ui.ColorBlack)}
	
	cChart.BarWidth=10
	dChart.BarWidth=10
	rChart.BarWidth=10
	
	cChart.PaddingBottom,cChart.PaddingLeft,cChart.PaddingRight, cChart.PaddingTop = 1,1,1,1
	dChart.PaddingBottom,dChart.PaddingLeft,dChart.PaddingRight, dChart.PaddingTop = 1,1,1,1
	rChart.PaddingBottom,rChart.PaddingLeft,rChart.PaddingRight, rChart.PaddingTop = 1,1,1,1
	
	
	
	labels := make([]string,0)
	
	if country == "all" {
		cMap := make(map[string]float64)
		dMap := make(map[string]float64)
		rMap := make(map[string]float64)
		index:=0
		for _, countryItem := range AllValues {
			j :=0
			for i:=len(countryItem)-1; i>=0; i-- {
				j++
				if index == 0 {
					labels = append(labels, countryItem[i].Date) // Add Labels only once
				}
				cMap[countryItem[i].Date] = cMap[countryItem[i].Date] + float64(countryItem[i].Confirmed)
				dMap[countryItem[i].Date] = dMap[countryItem[i].Date] + float64(countryItem[i].Deaths)
				rMap[countryItem[i].Date] = dMap[countryItem[i].Date] + float64(countryItem[i].Recovered)
				if j == max {
					break
				}
			}
			index++
		}
		for _,label := range labels {
			cData = append(cData, cMap[label])
			dData = append(dData, dMap[label])
			rData = append(rData, rMap[label])
		}
	}else{
		if countryItem, ok := AllValues[country]; ok {
			j:=0
			for i:=len(countryItem)-1; i>=0; i-- {
				j++
				labels = append(labels, countryItem[i].Date)
				cData = append(cData, float64(countryItem[i].Confirmed))
				dData = append(dData, float64(countryItem[i].Deaths))
				rData = append(rData, float64(countryItem[i].Recovered))
				if j == max {
					break
				}
			}
		} else {
			return errors.New("country not found in dataset")
		}
	}
	
	labels = ReverseStrings(labels)
	cData = ReverseFloats(cData)
	dData = ReverseFloats(dData)
	rData = ReverseFloats(rData)
	
	cChart.Labels = labels
	dChart.Labels = labels
	rChart.Labels = labels
	
	cChart.Data = cData
	dChart.Data = dData
	rChart.Data = rData
	
	p := widgets.NewParagraph()
	p.Text = "PRESS [q](fg:red) TO QUIT CHART"
	p.SetRect(0, 0, 25, 5)
	p.BorderStyle.Fg = ui.ColorYellow
	
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	
	grid := ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)
	
	grid.Set(
		ui.NewRow(1.0/10,
			ui.NewCol(1.0,p ),
			),
		ui.NewRow(3.0/10,
			ui.NewCol(1.0, cChart),
		),
		ui.NewRow(3.0/10,
			ui.NewCol(1.0, dChart),
		),
		ui.NewRow(3.0/10,
			ui.NewCol(1.0, rChart),
		),
	)
	
	ui.Render(grid)
	
	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return nil
		}
	}
}

func DrawCountryLineChart(country string, max int) error {
	if err := FetchData(false); err != nil {
		return err
	}
	
	plotChart := widgets.NewPlot()
	plotChart.Title = fmt.Sprintf("Case, Death, Recoveries Chart [%s]", country)
	
	plotChart.PaddingBottom,plotChart.PaddingLeft,plotChart.PaddingRight, plotChart.PaddingTop = 1,1,1,1
	plotChart.Data = make([][]float64, 3)
	plotChart.AxesColor = ui.ColorWhite
	plotChart.LineColors = []ui.Color{ui.ColorYellow, ui.ColorMagenta, ui.ColorGreen}
	plotChart.Marker = widgets.MarkerBraille
	plotChart.PlotType = widgets.LineChart
	plotChart.DrawDirection = widgets.DrawRight
	plotChart.HorizontalScale = 1
	plotChart.DataLabels = []string{"Cased", "Deaths", "Recovered"}
	
	if country == "all" {
		labels := make([]string,0)
		cMap := make(map[string]float64)
		dMap := make(map[string]float64)
		rMap := make(map[string]float64)
		index:=0
		for _, countryItem := range AllValues {
			j :=0
			for i:=len(countryItem)-1; i>=0; i-- {
				j++
				if index == 0 {
					labels = append(labels, countryItem[i].Date) // Add Labels only once
				}
				cMap[countryItem[i].Date] = cMap[countryItem[i].Date] + float64(countryItem[i].Confirmed)
				dMap[countryItem[i].Date] = dMap[countryItem[i].Date] + float64(countryItem[i].Deaths)
				rMap[countryItem[i].Date] = rMap[countryItem[i].Date] + float64(countryItem[i].Recovered)
				if j == max {
					break
				}
			}
			index++
		}
		for _,label := range labels {
			plotChart.Data[0] = append(plotChart.Data[0], cMap[label])
			plotChart.Data[1] = append(plotChart.Data[1], dMap[label])
			plotChart.Data[2] = append(plotChart.Data[2], rMap[label])
		}
	}else{
		if countryItem, ok := AllValues[country]; ok {
			j:=0
			for i:=len(countryItem)-1; i>=0; i-- {
				j++
				plotChart.Data[0] = append(plotChart.Data[0], float64(countryItem[i].Confirmed))
				plotChart.Data[1] = append(plotChart.Data[1], float64(countryItem[i].Deaths))
				plotChart.Data[2] = append(plotChart.Data[2], float64(countryItem[i].Recovered))
				if j == max {
					break
				}
			}
		} else {
			return errors.New("country not found in dataset")
		}
	}
	
	plotChart.Data[0] = ReverseFloats(plotChart.Data[0])
	plotChart.Data[1] = ReverseFloats(plotChart.Data[1])
	plotChart.Data[2] = ReverseFloats(plotChart.Data[2])
	
	p := widgets.NewParagraph()
	p.Text = "PRESS [q](fg:red) TO QUIT CHART | SERIES [Cases](fg:yellow), [Deaths](fg:magenta), [Recovered](fg:green)"
	p.SetRect(0, 0, 25, 5)
	p.BorderStyle.Fg = ui.ColorYellow
	
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	
	grid := ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)
	
	grid.Set(
		ui.NewRow(1.0/10,
			ui.NewCol(1.0,p ),
		),
		ui.NewRow(9.0/10,
			ui.NewCol(1.0, plotChart),
		),
	)
	
	ui.Render(grid)
	
	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return nil
		}
	}
}

func ReverseStrings(s []string)  []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func ReverseFloats(s []float64) []float64  {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}