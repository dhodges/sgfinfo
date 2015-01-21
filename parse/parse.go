package parse

import (
	"errors"
	"fmt"
	"strings"
)

type PlayerInfo struct {
	name string
	rank string
	team string
}

type GameInfo map[string]string

const BlackPlayerName = "PB"
const BlackPlayerRank = "BR"
const BlackPlayerTeam = "BT"
const WhitePlayerName = "PW"
const WhitePlayerRank = "WR"
const WhitePlayerTeam = "WT"
const Annotator = "AN"
const Copyright = "CP"
const Date = "DT"
const Event = "EV"
const GameComment = "GC"
const Comment = "C"
const GameName = "GN"
const Handicap = "HA"
const Opening = "ON"
const Overtime = "OT"
const Place = "PC"
const Result = "RE"
const Round = "RO"
const Rules = "RU"
const Source = "SO"
const TimeLimits = "TM"
const User = "US"
const Charset = "CA"
const Boardsize = "SZ"
const Komi = "KM"

type Property struct {
	name  string
	value string
}

func (p Property) String() string {
	return fmt.Sprintf("%s[%s]", p.name, p.value)
}

type Point struct {
	x rune
	y rune
}

type Node struct {
	point      Property
	properties []Property
	variations []*Node
	next       *Node
}

func (point *Point) String() string {
	return fmt.Sprintf("[%c%c]", point.x, point.y)
}

func (node *Node) String() string {
	attrs := []string{}
	attrs = append(attrs, node.point.String())
	for ndx := 0; ndx < len(node.properties); ndx += 1 {
		attrs = append(attrs, node.properties[ndx].String())
	}
	return strings.Join(attrs, "")
}

func (node *Node) AddProperty(prop Property) {
	switch prop.name {
	case "B", "W":
		node.point = prop
	default:
		node.properties = append(node.properties, prop)
	}
}

func (node *Node) NewNode() *Node {
	node.next = new(Node)
	return node.next
}

func (n *Node) NewVariation() *Node {
	node := new(Node)
	n.variations = append(n.variations, node)
	return node
}

type SGFGame struct {
	gameInfo GameInfo
	gameTree *Node
	errors   []error
}

func (gi *GameInfo) AddProperty(prop Property) {
	gi.properties[strings.ToUpper(prop.name)] = prop.value
}

func (gi *GameInfo) GetProperty(name string) (value string, ok bool) {
	value, ok = gi.properties[strings.ToUpper(name)]
	return value, ok
}

func (sgf *SGFGame) NodeCount() int {
	count := 0
	for node := sgf.gameTree; node != nil; node = node.next {
		count += 1
	}
	return count
}

func (sgf *SGFGame) NthNode(n int) (node *Node, err error) {
	if n < 1 {
		return nil, errors.New("n less than 1")
	}
	nodeCount := sgf.NodeCount()

	if n > nodeCount {
		return nil, errors.New(fmt.Sprintf("n greater than node count (%d)", nodeCount))
	}
	for node = sgf.gameTree; n > 1; n -= 1 {
		node = node.next
	}
	return node, nil
}

func (sgf *SGFGame) AddError(msg string) {
	sgf.errors = append(sgf.errors, errors.New(msg))
}

func (sgf *SGFGame) Parse(input string) *SGFGame {
	var currentNode *Node
	l := lex(input)
	prop := Property{}
	parsingSetup := false
	parsingGame := false
	nodeStack := new(Stack)

Loop:
	for {
		i := l.nextItem()
		switch i.typ {
		case itemLeftParen:
			if parsingGame {
				nodeStack.Push(currentNode)
				currentNode = currentNode.NewVariation()
			}
		case itemRightParen:
			if parsingGame {
				node := nodeStack.Pop()
				if node != nil {
					currentNode = node.(*Node)
				}
			}
		case itemSemiColon:
			if !parsingSetup && !parsingGame {
				parsingSetup = true
			} else {
				if !parsingGame {
					parsingSetup = false
					parsingGame = true
					sgf.gameTree = new(Node)
					currentNode = sgf.gameTree
				} else {
					currentNode = currentNode.NewNode()
				}
			}
		case itemPropertyName:
			prop = Property{i.val, ""}
		case itemPropertyValue:
			prop.value = i.val
			if parsingSetup {
				sgf.gameInfo.AddProperty(prop)
			} else {
				currentNode.AddProperty(prop)
			}
		case itemError:
			sgf.AddError(i.val)
			break Loop
		case itemEOF:
			break Loop
		}
	}
	return sgf
}
