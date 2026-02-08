package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type PricePoint struct {
	Minute int `json:"minute"`
	Price  int `json:"price"`
}

type HopeMetrics struct {
	WindowMinutes         int     `json:"windowMinutes"`
	LateJoinerProbability float64 `json:"lateJoinerProbability"`
}

type SimulationMetrics struct {
	PriceCurve                 []PricePoint   `json:"priceCurve"`
	MedianTimeToFirstStarByDay map[string]int `json:"medianTimeToFirstStarByJoinDay"`
	Hope                       HopeMetrics    `json:"hopeMetrics"`
	StarDistribution           map[int]int    `json:"starDistribution"`
}

type SimulationAssertions struct {
	StarPriceMonotonic bool `json:"starPriceMonotonic"`
	CoinBurnExact      bool `json:"coinBurnExact"`
	AdminEconomyLocked bool `json:"adminEconomyLocked"`
	HopeThresholdMet   bool `json:"hopeThresholdMet"`
}

type SimulationReport struct {
	SeasonID    string               `json:"seasonId"`
	Seed        int64                `json:"seed"`
	Generated   string               `json:"generatedAt"`
	Calibration CalibrationParams    `json:"calibration"`
	Metrics     SimulationMetrics    `json:"metrics"`
	Assertions  SimulationAssertions `json:"assertions"`
}

type SimPlayer struct {
	ID                 string
	Archetype          string
	JoinMinute         int
	Coins              int
	Stars              int
	DailyEarnTotal     int
	LastResetDay       int
	NextDailyMinute    int
	NextActivityMinute int
	FirstStarMinute    int
	ActiveStartMinute  int
	ActiveWindowMins   int
}

func RunSeasonSimulation(params CalibrationParams) (SimulationReport, error) {
	seasonMinutes := int(seasonLength().Minutes())
	rng := rand.New(rand.NewSource(params.Seed))

	players := buildSimPlayers(rng)

	globalCoinPool := 0
	coinsDistributed := 0
	coinsInWallets := 0
	starsPurchased := 0
	coinsBurned := 0
	totalCoinsSpent := 0
	emissionRemainder := 0.0
	marketPressure := 1.0
	simPriceFloor := params.P0

	last24 := make([]int, 24*60)
	last7d := make([]int, 7*24*60)
	window24Sum := 0
	window7Sum := 0

	priceCurve := []PricePoint{}
	lastPrice := 0
	priceMonotonic := true

	for minute := 0; minute < seasonMinutes; minute++ {
		secondsRemaining := int64((seasonMinutes - minute) * 60)
		price := ComputeStarPriceRaw(params, starsPurchased, int64(coinsInWallets), secondsRemaining, marketPressure)
		if price < simPriceFloor {
			price = simPriceFloor
		}
		if price < lastPrice {
			priceMonotonic = false
		}
		if price > simPriceFloor {
			simPriceFloor = price
		}
		lastPrice = price
		if minute%60 == 0 {
			priceCurve = append(priceCurve, PricePoint{Minute: minute, Price: price})
		}

		dailyTarget := EffectiveDailyEmissionTargetForParams(params, secondsRemaining, int64(coinsInWallets))
		coinsPerMinute := float64(dailyTarget) / (24 * 60)
		emissionRemainder += coinsPerMinute
		emitNow := int(emissionRemainder)
		if emitNow > 0 {
			emissionRemainder -= float64(emitNow)
			globalCoinPool += emitNow
		}

		minutePurchases := 0
		for i := range players {
			p := &players[i]
			if minute < p.JoinMinute {
				continue
			}

			day := minute / (24 * 60)
			if day != p.LastResetDay {
				p.DailyEarnTotal = 0
				p.LastResetDay = day
			}

			dailyCap := DailyEarnCapForParams(params, float64(minute)/float64(seasonMinutes))

			if minute >= p.NextDailyMinute {
				grant := minInt(params.DailyLoginReward, dailyCap-p.DailyEarnTotal)
				if grant > 0 && tryDistribute(&globalCoinPool, &coinsDistributed, grant) {
					p.Coins += grant
					coinsInWallets += grant
					p.DailyEarnTotal += grant
				}
				p.NextDailyMinute = minute + params.DailyLoginCooldownHours*60
			}

			dayMinute := minute % (24 * 60)
			active := dayMinute >= p.ActiveStartMinute && dayMinute < p.ActiveStartMinute+p.ActiveWindowMins
			if active && minute >= p.NextActivityMinute {
				grant := minInt(params.ActivityReward, dailyCap-p.DailyEarnTotal)
				if grant > 0 && tryDistribute(&globalCoinPool, &coinsDistributed, grant) {
					p.Coins += grant
					coinsInWallets += grant
					p.DailyEarnTotal += grant
				}
				p.NextActivityMinute = minute + maxInt(1, params.ActivityCooldownSeconds/60)
			}

			if active {
				price := ComputeStarPriceRaw(params, starsPurchased, int64(coinsInWallets), secondsRemaining, marketPressure)
				if price < simPriceFloor {
					price = simPriceFloor
				}
				buyQty := decidePurchaseQty(rng, p, price)
				if buyQty > 0 {
					cost := bulkCost(params, starsPurchased, int64(coinsInWallets), secondsRemaining, marketPressure, simPriceFloor, buyQty)
					if cost > 0 && p.Coins >= cost {
						p.Coins -= cost
						coinsInWallets -= cost
						p.Stars += buyQty
						starsPurchased += buyQty
						coinsBurned += cost
						totalCoinsSpent += cost
						minutePurchases += buyQty
						if p.FirstStarMinute < 0 {
							p.FirstStarMinute = minute - p.JoinMinute
						}
					}
				}
			}
		}

		updateSlidingWindows(last24, last7d, &window24Sum, &window7Sum, minute, minutePurchases)
		marketPressure = updateSimMarketPressure(marketPressure, window24Sum, window7Sum)
	}

	medianByBucket := medianFirstStarByBucket(players)
	lateHope := lateJoinerHope(players, 120)
	starDist := starDistribution(players)

	adminLocked := checkAdminLockdown()
	coinBurnExact := coinsBurned == totalCoinsSpent

	assertions := SimulationAssertions{
		StarPriceMonotonic: priceMonotonic,
		CoinBurnExact:      coinBurnExact,
		AdminEconomyLocked: adminLocked,
		HopeThresholdMet:   lateHope >= params.HopeThreshold,
	}

	report := SimulationReport{
		SeasonID:    params.SeasonID,
		Seed:        params.Seed,
		Generated:   time.Now().UTC().Format(time.RFC3339),
		Calibration: params,
		Metrics: SimulationMetrics{
			PriceCurve:                 priceCurve,
			MedianTimeToFirstStarByDay: medianByBucket,
			Hope: HopeMetrics{
				WindowMinutes:         120,
				LateJoinerProbability: lateHope,
			},
			StarDistribution: starDist,
		},
		Assertions: assertions,
	}

	return report, nil
}

func SaveSimulationReport(report SimulationReport, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "artifacts/simulations"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	filename := filepath.Join(outputDir, "season_simulation_"+time.Now().UTC().Format("20060102_150405")+".json")
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filename, bytes, 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func buildSimPlayers(rng *rand.Rand) []SimPlayer {
	players := []SimPlayer{}
	addPlayers := func(count int, archetype string, joinMinute int, activeWindow int) {
		for i := 0; i < count; i++ {
			players = append(players, SimPlayer{
				ID:                 archetype + "-" + randomID(rng),
				Archetype:          archetype,
				JoinMinute:         joinMinute,
				Coins:              0,
				Stars:              0,
				DailyEarnTotal:     0,
				LastResetDay:       joinMinute / (24 * 60),
				NextDailyMinute:    joinMinute,
				NextActivityMinute: joinMinute,
				FirstStarMinute:    -1,
				ActiveStartMinute:  rng.Intn(24 * 60),
				ActiveWindowMins:   activeWindow,
			})
		}
	}

	addPlayers(12, "early_buyer", 0, 240)
	addPlayers(10, "cautious", 0, 180)
	addPlayers(6, "whale", 0, 300)
	addPlayers(12, "random", 0, 150)
	addPlayers(16, "late_joiner", 21*24*60, 180)

	return players
}

func decidePurchaseQty(rng *rand.Rand, p *SimPlayer, price int) int {
	switch p.Archetype {
	case "early_buyer":
		if p.Coins >= price {
			return 1
		}
	case "late_joiner":
		if p.Coins >= price {
			return 1
		}
	case "whale":
		if p.Coins >= price*3 {
			return 3
		}
	case "cautious":
		if p.Coins >= price && price <= 60 {
			return 1
		}
	case "random":
		if p.Coins >= price && rng.Float64() < 0.002 {
			return 1
		}
	}
	return 0
}

func bulkCost(params CalibrationParams, starsPurchased int, coinsInCirculation int64, secondsRemaining int64, marketPressure float64, priceFloor int, qty int) int {
	total := 0
	for i := 0; i < qty; i++ {
		base := ComputeStarPriceRaw(params, starsPurchased+i, coinsInCirculation, secondsRemaining, marketPressure)
		if base < priceFloor {
			base = priceFloor
		}
		multiplier := 1 + params.Gamma*float64(i*i)
		// Round UP (ceil) for prices to ensure conservative charging
		price := int(math.Ceil(float64(base) * multiplier))
		total += price
	}
	return total
}

func updateSlidingWindows(last24 []int, last7 []int, sum24 *int, sum7 *int, minute int, value int) {
	idx24 := minute % len(last24)
	idx7 := minute % len(last7)
	*sum24 -= last24[idx24]
	*sum7 -= last7[idx7]
	last24[idx24] = value
	last7[idx7] = value
	*sum24 += value
	*sum7 += value
}

func updateSimMarketPressure(current float64, last24 int, last7 int) float64 {
	longTermDaily := float64(last7) / 7.0
	if longTermDaily < 1 {
		longTermDaily = 1
	}
	ratio := float64(last24) / longTermDaily
	desired := 1.0
	if ratio >= 1 {
		desired = 1 + math.Min(0.8, 0.25*(ratio-1))
	} else {
		desired = 1 - math.Min(0.3, 0.15*(1-ratio))
	}
	maxDelta := 0.02 / 60
	delta := desired - current
	if delta > maxDelta {
		delta = maxDelta
	}
	if delta < -maxDelta {
		delta = -maxDelta
	}
	current += delta
	if current < 0.6 {
		current = 0.6
	}
	if current > 1.8 {
		current = 1.8
	}
	return current
}

func medianFirstStarByBucket(players []SimPlayer) map[string]int {
	buckets := map[string][]int{}
	for _, p := range players {
		bucket := joinDayBucket(p.JoinMinute)
		if p.FirstStarMinute >= 0 {
			buckets[bucket] = append(buckets[bucket], p.FirstStarMinute)
		}
	}

	result := map[string]int{}
	for bucket, values := range buckets {
		sort.Ints(values)
		mid := len(values) / 2
		if len(values) == 0 {
			continue
		}
		result[bucket] = values[mid]
	}
	return result
}

func lateJoinerHope(players []SimPlayer, windowMinutes int) float64 {
	lateTotal := 0
	lateSuccess := 0
	for _, p := range players {
		if p.Archetype != "late_joiner" {
			continue
		}
		lateTotal++
		if p.FirstStarMinute >= 0 && p.FirstStarMinute <= windowMinutes {
			lateSuccess++
		}
	}
	if lateTotal == 0 {
		return 0
	}
	return float64(lateSuccess) / float64(lateTotal)
}

func starDistribution(players []SimPlayer) map[int]int {
	dist := map[int]int{}
	for _, p := range players {
		dist[p.Stars]++
	}
	return dist
}

func joinDayBucket(joinMinute int) string {
	day := joinMinute / (24 * 60)
	if day >= 21 {
		return "day21+"
	}
	if day >= 14 {
		return "day14"
	}
	if day >= 7 {
		return "day7"
	}
	return "day0"
}

func tryDistribute(pool *int, distributed *int, amount int) bool {
	available := *pool - *distributed
	if amount <= 0 || available < amount {
		return false
	}
	*distributed += amount
	return true
}

func randomID(rng *rand.Rand) string {
	letters := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func checkAdminLockdown() bool {
	req := httptest.NewRequest(http.MethodPost, "/admin/economy", nil)
	rr := httptest.NewRecorder()
	adminEconomyHandler(nil)(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		return false
	}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/settings", nil)
	rr2 := httptest.NewRecorder()
	adminSettingsHandler(nil)(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		return false
	}
	return true
}
