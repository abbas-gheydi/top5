package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

var lables = []string{
	"CPU",
	"RAM",
	"LOAD",
}

type netstat struct {
	sent     string
	recieve  string
	dropin   string
	dropout  string
	errorIn  string
	errorOut string
}

func getNetStat() (stats netstat) {
	ioc, _ := net.IOCounters(false)
	stats.sent = fmt.Sprint(ioc[0].BytesSent / 1024 / 1024)
	stats.recieve = fmt.Sprint(ioc[0].BytesRecv / 1024 / 1024)
	stats.dropin = fmt.Sprint(ioc[0].Dropin)
	stats.dropout = fmt.Sprint(ioc[0].Dropout)
	stats.errorIn = fmt.Sprint(ioc[0].Errin)
	stats.errorOut = fmt.Sprint(ioc[0].Errout)

	return
}

func getOneMinSysLoad() int {
	l, _ := load.Avg()
	cores, _ := cpu.Counts(true)
	oneMinLoad := (l.Load1 / float64(cores)) * 100
	return int(oneMinLoad)

}

type hardDisk struct {
	name string
	size int
}

func getDiskUsage() []hardDisk {
	var listOfDisks []hardDisk
	parts, _ := disk.Partitions(false)
	for _, i := range parts {
		if !strings.Contains(i.Device, "loop") {
			d, _ := disk.Usage(i.Mountpoint)
			listOfDisks = append(listOfDisks,
				hardDisk{name: i.Mountpoint, size: int(d.UsedPercent)})
		}

	}
	return listOfDisks
}

func addDisksNamesToBarCharts() {
	disks := getDiskUsage()
	for _, i := range disks {
		lables = append(lables, "du:"+i.name)
	}
}

func getCpuUsage() int {
	percent, _ := cpu.Percent(time.Second, false)

	return int(percent[0])
}

func getMemoryUsage() int {
	v, _ := mem.VirtualMemory()
	return int(v.UsedPercent)

}

// playBarChart continuously changes the displayed values on the bar chart once every delay.
// Exits when the context expires.
func playBarChart(ctx context.Context, bc *barchart.BarChart, delay time.Duration) {
	const max = 100

	ticker := time.NewTicker(delay)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:

			var values []int

			//Add charts
			values = append(values, getCpuUsage())
			values = append(values, getMemoryUsage())
			values = append(values, getOneMinSysLoad()) //load
			//addDIskUsageToBarValues
			for _, i := range getDiskUsage() {
				values = append(values, i.size)
			}

			if err := bc.Values(values, max); err != nil {
				panic(err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func writeLines(ctx context.Context, t *text.Text, delay time.Duration) {

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			networkStats := getNetStat()

			t.Reset()
			if err := t.Write(fmt.Sprint("Total Sent: ", networkStats.sent, " MB\n",
				"Total Received: ", networkStats.recieve, " MB\n",
				"Total Incoming drops: ", networkStats.dropin, " packet\n",
				"Total Incoming errors: ", networkStats.errorIn, " packet\n",
				"Total Outgoing drops: ", networkStats.dropout, " packet\n",
				"Total Outgoing errors: ", networkStats.errorOut, " packet\n",
			)); err != nil {
				panic(err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func main() {

	t, err := tcell.New()
	if err != nil {
		panic(err)
	}
	defer t.Close()

	ctx, cancel := context.WithCancel(context.Background())
	addDisksNamesToBarCharts()

	bc, err := barchart.New(
		barchart.BarColors([]cell.Color{
			cell.ColorBlue,
			cell.ColorRed,
			cell.ColorYellow,
			cell.ColorBlue,
			cell.ColorGreen,
			cell.ColorRed,
		}),
		barchart.ValueColors([]cell.Color{
			cell.ColorRed,
			cell.ColorYellow,
			cell.ColorNumber(33),
			cell.ColorGreen,
			cell.ColorRed,
			cell.ColorNumber(33),
		}),
		barchart.ShowValues(),
		barchart.BarWidth(8),
		barchart.Labels(lables),
	)
	if err != nil {
		panic(err)
	}

	textWidget, err := text.New()
	if err != nil {
		panic(err)
	}
	if err := textWidget.Write("Loading data ..."); err != nil {
		panic(err)
	}

	go playBarChart(ctx, bc, 1*time.Second)
	go writeLines(ctx, textWidget, 1*time.Second)

	c, err := container.New(
		t,
		container.Border(linestyle.Light),
		container.BorderColor(cell.ColorGreen),
		container.BorderTitle("PRESS Q TO QUIT"),

		container.SplitHorizontal(
			container.Top(

				container.BorderTitle("Resource Utilization Percentage"),

				container.Border(linestyle.Light),
				container.PlaceWidget(bc)),

			container.Bottom(
				container.Border(linestyle.Light),
				container.BorderTitle("Networks Statistics"),
				container.PlaceWidget(textWidget),
			)),
	)
	if err != nil {
		panic(err)
	}

	quitter := func(k *terminalapi.Keyboard) {
		if k.Key == 'q' || k.Key == 'Q' {
			cancel()
		}
	}

	if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter)); err != nil {
		panic(err)
	}
}
