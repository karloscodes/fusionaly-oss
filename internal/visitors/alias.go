package visitors

import "hash/fnv"

// The words of a visitor alias. Keep them neutral, one word each, with no
// word in both lists. 64 x 64 gives 4096 names.
var visitorAdjectives = []string{
	// Colors
	"Amber", "Azure", "Bronze", "Cobalt", "Copper", "Coral", "Cream", "Crimson",
	"Cyan", "Golden", "Hazel", "Indigo", "Ivory", "Jade", "Lemon", "Lilac",
	"Lime", "Mint", "Ochre", "Olive", "Pearl", "Plum", "Rosy", "Ruby",
	"Russet", "Saffron", "Sage", "Scarlet", "Silver", "Slate", "Teal", "Violet",
	// Qualities
	"Agile", "Bold", "Brave", "Bright", "Brisk", "Calm", "Clever", "Cozy",
	"Curious", "Eager", "Fair", "Gentle", "Glad", "Happy", "Humble", "Jolly",
	"Keen", "Kind", "Lively", "Lucky", "Merry", "Mellow", "Nimble", "Noble",
	"Patient", "Polite", "Quiet", "Steady", "Sunny", "Swift", "Tidy", "Witty",
}

var visitorAnimals = []string{
	"Alpaca", "Badger", "Bear", "Beaver", "Bison", "Camel", "Crane", "Deer",
	"Dolphin", "Dove", "Duck", "Eagle", "Elk", "Falcon", "Finch", "Fox",
	"Gazelle", "Gecko", "Goose", "Hare", "Hawk", "Hedgehog", "Heron", "Ibis",
	"Kiwi", "Koala", "Lark", "Lemur", "Lion", "Llama", "Lynx", "Magpie",
	"Marten", "Mole", "Moose", "Newt", "Orca", "Osprey", "Otter", "Owl",
	"Panda", "Parrot", "Pelican", "Penguin", "Puffin", "Quail", "Rabbit", "Raven",
	"Robin", "Salmon", "Seal", "Sparrow", "Squirrel", "Stork", "Swan", "Tiger",
	"Toucan", "Turtle", "Walrus", "Whale", "Wolf", "Wren", "Yak", "Zebra",
}

// VisitorAlias returns an anonymized display name for the given visitor signature.
func VisitorAlias(signature string) string {
	h := fnv.New32a()
	h.Write([]byte(signature))
	index := int(h.Sum32())

	adjIndex := index % len(visitorAdjectives)
	animalIndex := (index / len(visitorAdjectives)) % len(visitorAnimals)

	return visitorAdjectives[adjIndex] + " " + visitorAnimals[animalIndex]
}
