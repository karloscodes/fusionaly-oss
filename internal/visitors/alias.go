package visitors

import "hash/fnv"

var visitorAdjectives = []string{
	// Adventurous
	"Curious", "Happy", "Clever", "Wise", "Playful", "Brave", "Swift", "Gentle", "Smart", "Busy",
	"Adventurous", "Daring", "Fearless", "Bold", "Courageous", "Energetic", "Lively", "Spirited", "Vibrant", "Dynamic",
	"Agile", "Nimble", "Quick", "Fast", "Speedy", "Rapid", "Fleet", "Swift", "Brisk", "Zippy",
	"Bright", "Brilliant", "Shining", "Radiant", "Glowing", "Luminous", "Sparkling", "Dazzling", "Gleaming", "Beaming",
	"Cheerful", "Joyful", "Merry", "Jolly", "Blissful", "Delighted", "Ecstatic", "Gleeful", "Jubilant", "Thrilled",
	"Creative", "Imaginative", "Innovative", "Inventive", "Artistic", "Original", "Resourceful", "Clever", "Ingenious", "Brilliant",
	"Elegant", "Graceful", "Refined", "Sophisticated", "Polished", "Stylish", "Chic", "Classy", "Dapper", "Dashing",
	"Friendly", "Kind", "Warm", "Welcoming", "Affable", "Amiable", "Cordial", "Genial", "Gracious", "Hospitable",
	"Magical", "Mystical", "Enchanting", "Charming", "Spellbinding", "Captivating", "Mesmerizing", "Fascinating", "Alluring", "Bewitching",
	"Peaceful", "Calm", "Serene", "Tranquil", "Placid", "Quiet", "Still", "Composed", "Relaxed", "Soothing",
}

var visitorAnimals = []string{
	// Land Animals
	"Panda", "Fox", "Owl", "Otter", "Lion", "Eagle", "Deer", "Raven", "Beaver", "Koala",
	"Sloth", "Hamster", "Cat", "Bear", "Penguin", "Kangaroo", "Parrot", "Giraffe", "Duck", "Raccoon",
	"Elephant", "Monkey", "Hyena", "Gorilla", "Leopard", "Camel", "Jerboa", "Meerkat", "Scorpion", "Goat",
	"Condor", "Sheep", "Llama", "Mouse", "Bee", "Squirrel", "Rabbit", "Hedgehog", "Dragon", "Unicorn",
	"Phoenix", "Griffin", "Pegasus", "Tiger", "Wolf", "Falcon", "Hawk", "Owl", "Eagle", "Falcon",
	// Sea Creatures
	"Dolphin", "Whale", "Seahorse", "Eel", "Jellyfish", "Turtle", "Octopus", "Squid", "Shark", "Ray",
	"Seal", "Walrus", "Penguin", "Crab", "Lobster", "Starfish", "Coral", "Anemone", "Clam", "Oyster",
	// Mythical Creatures
	"Dragon", "Unicorn", "Phoenix", "Griffin", "Pegasus", "Mermaid", "Centaur", "Sphinx", "Chimera", "Kraken",
	"Yeti", "Bigfoot", "LochNess", "Kitsune", "Tengu", "Banshee", "Valkyrie", "Minotaur", "Hydra", "Cerberus",
	// Birds
	"Eagle", "Falcon", "Hawk", "Owl", "Raven", "Parrot", "Peacock", "Swan", "Crane", "Heron",
	"Kingfisher", "Hummingbird", "Woodpecker", "Nightingale", "Lark", "Finch", "Sparrow", "Dove", "Pigeon", "Crow",
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
