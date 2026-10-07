package user_agent

import (
	"embed"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2/v2"
	"gopkg.in/yaml.v3"
)

type UserAgent struct {
	UserAgent string
	OS        string
	Browser   string
	Device    string
	Mobile    bool
	Tablet    bool
	Desktop   bool
	Bot       bool
}

// Embed the database files
//
//go:embed database/bots.yml
//go:embed database/oss.yml
//go:embed database/vendorfragments.yml
//go:embed database/client/browser_engine.yml
//go:embed database/client/browsers.yml
//go:embed database/client/feed_readers.yml
//go:embed database/client/libraries.yml
//go:embed database/client/mediaplayers.yml
//go:embed database/client/mobile_apps.yml
//go:embed database/client/pim.yml
//go:embed database/client/hints/apps.yml
//go:embed database/client/hints/browsers.yml
//go:embed database/device/cameras.yml
//go:embed database/device/car_browsers.yml
//go:embed database/device/consoles.yml
//go:embed database/device/mobiles.yml
//go:embed database/device/notebooks.yml
//go:embed database/device/portable_media_player.yml
//go:embed database/device/shell_tv.yml
//go:embed database/device/televisions.yml
var databaseFiles embed.FS

// Browser entry structure
type BrowserEntry struct {
	Regex   string `yaml:"regex"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Engine  struct {
		Default  string            `yaml:"default"`
		Versions map[string]string `yaml:"versions"`
	} `yaml:"engine"`
}

// OS entry structure
type OSEntry struct {
	Regex   string `yaml:"regex"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Device model structure
type DeviceModel struct {
	Regex  string `yaml:"regex"`
	Model  string `yaml:"model"`
	Device string `yaml:"device"` // overrides the brand's device type, as in Matomo
}

// Device entry structure
type DeviceEntry struct {
	Regex  string        `yaml:"regex"`
	Device string        `yaml:"device"`
	Model  string        `yaml:"model"`
	Models []DeviceModel `yaml:"models"`
}

// Bot entry structure
type BotEntry struct {
	Regex    string `yaml:"regex"`
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	URL      string `yaml:"url"`
	Producer struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"producer"`
}

// matchTimeout bounds one match. A user agent is untrusted input, and a
// backtracking engine can take very long on a crafted one; past the limit
// the pattern counts as no match.
const matchTimeout = 100 * time.Millisecond

// Compiled regex cache. The database uses PCRE syntax. Go's regexp compiles
// 99% of the patterns and matches in linear time; the rest use lookarounds,
// which only regexp2, a backtracking engine, supports.
type RegexCache struct {
	compiled map[string]*pattern
	mutex    sync.RWMutex
}

// pattern is one compiled database regex: re2 when Go's regexp compiles it,
// otherwise pcre.
type pattern struct {
	re2  *regexp.Regexp
	pcre *regexp2.Regexp
}

func newRegexCache() *RegexCache {
	return &RegexCache{
		compiled: make(map[string]*pattern),
	}
}

func (rc *RegexCache) get(pattern string) (*pattern, error) {
	rc.mutex.RLock()
	if regex, exists := rc.compiled[pattern]; exists {
		rc.mutex.RUnlock()
		return regex, nil
	}
	rc.mutex.RUnlock()

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Double-check pattern
	if regex, exists := rc.compiled[pattern]; exists {
		return regex, nil
	}

	compiled, err := compilePattern(pattern)
	if err != nil {
		return nil, err
	}
	rc.compiled[pattern] = compiled
	return compiled, nil
}

func compilePattern(expr string) (*pattern, error) {
	if re, err := regexp.Compile(expr); err == nil {
		return &pattern{re2: re}, nil
	}
	// Keep PCRE's group numbers, so $1 in the database means the same group.
	re, err := regexp2.Compile(expr, regexp2.OptionMaintainCaptureOrder())
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = matchTimeout
	return &pattern{pcre: re}, nil
}

// matchString reports whether p matches s.
func matchString(p *pattern, s string) bool {
	if p.re2 != nil {
		return p.re2.MatchString(s)
	}
	ok, err := p.pcre.MatchString(s)
	return err == nil && ok
}

// findStringSubmatch returns the match and its groups, or nil when p does
// not match. A group that did not take part is "".
func findStringSubmatch(p *pattern, s string) []string {
	if p.re2 != nil {
		// Most patterns do not match. A yes/no check skips the capture
		// work, so check that first.
		if !p.re2.MatchString(s) {
			return nil
		}
		return p.re2.FindStringSubmatch(s)
	}
	m, err := p.pcre.FindStringMatch(s)
	if err != nil || m == nil {
		return nil
	}
	groups := m.Groups()
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.String()
	}
	return out
}

// Global parser instance
var (
	parser *DeviceDetectorParser
	once   sync.Once
)

type DeviceDetectorParser struct {
	browsers   []BrowserEntry
	oss        []OSEntry
	devices    []brandEntry // in file order: the first brand that matches wins
	bots       []BotEntry
	regexCache *RegexCache
}

// brandEntry is one brand of a device file.
type brandEntry struct {
	Brand string
	DeviceEntry
}

func getParser() *DeviceDetectorParser {
	once.Do(func() {
		parser = &DeviceDetectorParser{
			regexCache: newRegexCache(),
		}

		// Load browsers
		if data, err := databaseFiles.ReadFile("database/client/browsers.yml"); err == nil {
			if err := yaml.Unmarshal(data, &parser.browsers); err != nil {
				fmt.Printf("Error parsing browsers.yml: %v\n", err)
			}
		}

		// Load OS
		if data, err := databaseFiles.ReadFile("database/oss.yml"); err == nil {
			if err := yaml.Unmarshal(data, &parser.oss); err != nil {
				fmt.Printf("Error parsing oss.yml: %v\n", err)
			}
		}

		// Load bots
		if data, err := databaseFiles.ReadFile("database/bots.yml"); err == nil {
			if err := yaml.Unmarshal(data, &parser.bots); err != nil {
				fmt.Printf("Error parsing bots.yml: %v\n", err)
			}
		}

		// Load devices from multiple files. Matomo checks TVs and notebooks
		// first, but only when the user agent carries their marker (HbbTV,
		// FBMD). Without those checks, mobiles must come first.
		deviceFiles := []string{
			"database/device/mobiles.yml",
			"database/device/notebooks.yml",
			"database/device/televisions.yml",
			"database/device/consoles.yml",
			"database/device/cameras.yml",
			"database/device/car_browsers.yml",
			"database/device/portable_media_player.yml",
			"database/device/shell_tv.yml",
		}

		for _, file := range deviceFiles {
			if data, err := databaseFiles.ReadFile(file); err == nil {
				brands, err := loadBrands(data)
				if err != nil {
					fmt.Printf("Error parsing %s: %v\n", file, err)
				}
				parser.devices = append(parser.devices, brands...)
			}
		}
	})
	return parser
}

func (p *DeviceDetectorParser) parseBot(userAgent string) *BotEntry {
	for _, bot := range p.bots {
		if regex, err := p.regexCache.get(bot.Regex); err == nil {
			if matchString(regex, userAgent) {
				return &bot
			}
		}
	}
	return nil
}

func (p *DeviceDetectorParser) parseBrowser(userAgent string) (string, string) {
	for _, entry := range p.browsers {
		if regex, err := p.regexCache.get(entry.Regex); err == nil {
			if matches := findStringSubmatch(regex, userAgent); len(matches) > 0 {
				version := ""
				if entry.Version != "" && len(matches) > 1 {
					// Replace $1, $2, etc. with actual match groups
					version = entry.Version
					for i, match := range matches[1:] {
						placeholder := fmt.Sprintf("$%d", i+1)
						version = strings.ReplaceAll(version, placeholder, match)
					}
				}
				return entry.Name, version
			}
		}
	}
	return "Unknown", ""
}

func (p *DeviceDetectorParser) parseOS(userAgent string) (string, string) {
	for _, entry := range p.oss {
		if regex, err := p.regexCache.get(entry.Regex); err == nil {
			if matches := findStringSubmatch(regex, userAgent); len(matches) > 0 {
				name := entry.Name
				version := ""
				if len(matches) > 1 {
					// Replace $1, $2, etc. with actual match groups
					for i, match := range matches[1:] {
						placeholder := fmt.Sprintf("$%d", i+1)
						name = strings.ReplaceAll(name, placeholder, match)
					}
					if entry.Version != "" {
						version = entry.Version
						for i, match := range matches[1:] {
							placeholder := fmt.Sprintf("$%d", i+1)
							version = strings.ReplaceAll(version, placeholder, match)
						}
					}
				}
				return name, version
			}
		}
	}
	return "Unknown", ""
}

// loadBrands reads a device file in file order. A Go map would give a
// random order, and with it a random brand when two brands match one user
// agent.
func loadBrands(data []byte) ([]brandEntry, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("want a mapping of brands")
	}
	pairs := doc.Content[0].Content
	brands := make([]brandEntry, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		var entry DeviceEntry
		if err := pairs[i+1].Decode(&entry); err != nil {
			return nil, fmt.Errorf("brand %s: %w", pairs[i].Value, err)
		}
		brands = append(brands, brandEntry{Brand: pairs[i].Value, DeviceEntry: entry})
	}
	return brands, nil
}

// devicePattern wraps a device regex the way Matomo's device parser does:
// case-insensitive, and the match must not start inside a word. Without
// the wrapper, the "iPhone X" model's " X)" matches "Mac OS X)" in every
// iOS user agent.
func devicePattern(regex string) string {
	return `(?i)(?:^|[^A-Z0-9\-_]|[^A-Z0-9\-]_|sprd-|MZ-)(?:` + regex + `)`
}

func (p *DeviceDetectorParser) parseDevice(userAgent string) (string, string, bool, bool, bool) {
	for _, entry := range p.devices {
		brand := entry.Brand
		if regex, err := p.regexCache.get(devicePattern(entry.Regex)); err == nil {
			if matches := findStringSubmatch(regex, userAgent); len(matches) > 0 {
				deviceType := entry.Device
				if deviceType == "" {
					deviceType = "Unknown"
				}

				model := ""

				// Check for specific model matches
				if len(entry.Models) > 0 {
					for _, modelEntry := range entry.Models {
						if modelRegex, err := p.regexCache.get(devicePattern(modelEntry.Regex)); err == nil {
							if modelMatches := findStringSubmatch(modelRegex, userAgent); len(modelMatches) > 0 {
								model = modelEntry.Model
								if modelEntry.Device != "" {
									deviceType = modelEntry.Device
								}
								// Replace $1, $2, etc. with actual match groups
								if len(modelMatches) > 1 {
									for i, match := range modelMatches[1:] {
										placeholder := fmt.Sprintf("$%d", i+1)
										model = strings.ReplaceAll(model, placeholder, match)
									}
								}
								break
							}
						}
					}
				}

				// If no specific model found, use the generic model
				if model == "" && entry.Model != "" {
					model = entry.Model
					// Replace $1, $2, etc. with actual match groups from main regex
					if len(matches) > 1 {
						for i, match := range matches[1:] {
							placeholder := fmt.Sprintf("$%d", i+1)
							model = strings.ReplaceAll(model, placeholder, match)
						}
					}
				}

				// Default to brand if no model
				if model == "" {
					model = brand
				}

				// Determine device characteristics
				mobile := deviceType == "smartphone" || deviceType == "feature phone" || deviceType == "phablet"
				tablet := deviceType == "tablet"
				desktop := deviceType == "desktop" || deviceType == "notebook"

				return brand, model, mobile, tablet, desktop
			}
		}
	}

	// Fallback device detection based on user agent patterns
	ua := strings.ToLower(userAgent)

	// Check for tablet indicators first (they often contain "mobile" too)
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "Tablet", "Tablet Device", false, true, false
	}

	// Check for mobile indicators
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") ||
		strings.Contains(ua, "iphone") || strings.Contains(ua, "ipod") ||
		strings.Contains(ua, "blackberry") || strings.Contains(ua, "windows phone") {
		return "Mobile", "Mobile Device", true, false, false
	}

	// Default to desktop
	return "Desktop", "Desktop Device", false, false, true
}

// headlessBrowsers run pages without a person: monitors, scrapers, and test
// runners. The device database lists them as browsers.
var headlessBrowsers = []string{"HeadlessChrome", "PhantomJS"}

func isHeadless(userAgent string) bool {
	for _, name := range headlessBrowsers {
		if strings.Contains(userAgent, name) {
			return true
		}
	}
	return false
}

// parsedCacheSize bounds the cache of parsed user agents. Parsing checks
// thousands of patterns, and real traffic repeats the same few hundred user
// agents.
const parsedCacheSize = 10000

var parsedCache = struct {
	sync.Mutex
	byAgent map[string]UserAgent
}{byAgent: make(map[string]UserAgent)}

// ParseUserAgent returns the browser, OS, and device of a user agent.
func ParseUserAgent(userAgent string) UserAgent {
	parsedCache.Lock()
	result, ok := parsedCache.byAgent[userAgent]
	parsedCache.Unlock()
	if ok {
		return result
	}

	result = parseUserAgent(userAgent)

	parsedCache.Lock()
	if len(parsedCache.byAgent) >= parsedCacheSize {
		clear(parsedCache.byAgent)
	}
	parsedCache.byAgent[userAgent] = result
	parsedCache.Unlock()
	return result
}

func parseUserAgent(userAgent string) UserAgent {
	parser := getParser()

	if isHeadless(userAgent) {
		return UserAgent{UserAgent: userAgent, OS: "Unknown", Browser: "Headless", Device: "Bot", Bot: true}
	}

	// Check for bots first
	if bot := parser.parseBot(userAgent); bot != nil {
		return UserAgent{
			UserAgent: userAgent,
			OS:        "Unknown",
			Browser:   bot.Name,
			Device:    "Bot",
			Mobile:    false,
			Tablet:    false,
			Desktop:   false,
			Bot:       true,
		}
	}

	browser, _ := parser.parseBrowser(userAgent)
	os, _ := parser.parseOS(userAgent)
	brand, _, mobile, tablet, desktop := parser.parseDevice(userAgent)

	return UserAgent{
		UserAgent: userAgent,
		OS:        os,
		Browser:   browser,
		Device:    brand,
		Mobile:    mobile,
		Tablet:    tablet,
		Desktop:   desktop,
		Bot:       false,
	}
}
