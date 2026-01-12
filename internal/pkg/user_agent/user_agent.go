package user_agent

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"go.elara.ws/pcre"
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
	Regex string `yaml:"regex"`
	Model string `yaml:"model"`
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

// Compiled regex cache
type RegexCache struct {
	compiled map[string]*pcre.Regexp
	mutex    sync.RWMutex
}

func newRegexCache() *RegexCache {
	return &RegexCache{
		compiled: make(map[string]*pcre.Regexp),
	}
}

func (rc *RegexCache) get(pattern string) (*pcre.Regexp, error) {
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

	regex, err := pcre.Compile(pattern)
	if err != nil {
		return nil, err
	}
	rc.compiled[pattern] = regex
	return regex, nil
}

// Global parser instance
var (
	parser *DeviceDetectorParser
	once   sync.Once
)

type DeviceDetectorParser struct {
	browsers   []BrowserEntry
	oss        []OSEntry
	devices    map[string]DeviceEntry
	bots       []BotEntry
	regexCache *RegexCache
}

func getParser() *DeviceDetectorParser {
	once.Do(func() {
		parser = &DeviceDetectorParser{
			regexCache: newRegexCache(),
			devices:    make(map[string]DeviceEntry),
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

		// Load devices from multiple files
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
				var brands map[string]DeviceEntry
				if err := yaml.Unmarshal(data, &brands); err == nil {
					for brand, entry := range brands {
						parser.devices[brand] = entry
					}
				}
			}
		}
	})
	return parser
}

func (p *DeviceDetectorParser) parseBot(userAgent string) *BotEntry {
	for _, bot := range p.bots {
		if regex, err := p.regexCache.get(bot.Regex); err == nil {
			if regex.MatchString(userAgent) {
				return &bot
			}
		}
	}
	return nil
}

func (p *DeviceDetectorParser) parseBrowser(userAgent string) (string, string) {
	for _, entry := range p.browsers {
		if regex, err := p.regexCache.get(entry.Regex); err == nil {
			if matches := regex.FindStringSubmatch(userAgent); len(matches) > 0 {
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
			if matches := regex.FindStringSubmatch(userAgent); len(matches) > 0 {
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

func (p *DeviceDetectorParser) parseDevice(userAgent string) (string, string, bool, bool, bool) {
	for brand, entry := range p.devices {
		if regex, err := p.regexCache.get(entry.Regex); err == nil {
			if matches := regex.FindStringSubmatch(userAgent); len(matches) > 0 {
				deviceType := entry.Device
				if deviceType == "" {
					deviceType = "Unknown"
				}

				model := ""

				// Check for specific model matches
				if len(entry.Models) > 0 {
					for _, modelEntry := range entry.Models {
						if modelRegex, err := p.regexCache.get(modelEntry.Regex); err == nil {
							if modelMatches := modelRegex.FindStringSubmatch(userAgent); modelMatches != nil {
								model = modelEntry.Model
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

func ParseUserAgent(userAgent string) UserAgent {
	parser := getParser()

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
