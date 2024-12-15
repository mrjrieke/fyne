package dialog

import (
	"strings"

	"fyne.io/fyne/v2"
)

const (
	// TODO: convert to iota when comfortable with events to save stack space.
	// CreateContent - create content in window.
	CreateContentRequest string = "CreateContentRequest"
	// RefreshContent - refresh content in window.
	RefreshContent string = "RefreshContent"
	// RefreshTickerContent - refresh popup ticker content
	RefreshTickerContent string = "RefreshTickerContent"
	// ItemSelectedRequest - an item was selected
	ItemSelectedRequest string = "ItemSelectedRequest"
)

const (
	TickerContent        string = "TickerContent"
	TickerSearchContent  string = "TickerSearchContent"
	SearchQueryContent   string = "SearchQueryContent"
	PageContent          string = "PageContent"
)

type TagType int

type TextTag struct {
	LeaderIndex int
	ContentIndex int
	TagLength   int // Optional -- some tag values are in the text already like CharacterQuote
	TagValue   string
	TagType    TagType
}

// TextStack is a search result returned by the search engine.
type TextStack struct {
	Body     strings.Builder
	TextTags []TextTag
}

type SearchEnvelope struct {
	SearchQuery string
	QueryType   string
	SelectionStack []string // Stack of selections
	                        // This is a sort of context of entries the user
							// has selected in the drilldown.
//	SelectionDepth int // Indicates current user 'selection level'  0 is top
	SearchResult *TextStack
}

// ContentEvent defines the parameters of a content event
type ContentEvent struct {
	SourceWidgetId  uint32
	DestinationWidgetIds []uint32
	ContentAction string
	ContentType   string
	Content       *TextStack
}

// ContentEvent defines the parameters of a content event
type TiniPointEvent struct {
	PointEvent *fyne.PointEvent
	SourceWidgetId  uint32
	DestinationWidgetIds []uint32
}

type ContentConsumer interface {
	GetId() uint32
	ContentChanged(*ContentEvent)
	GetSelected(*fyne.PointEvent) string
}

type TappedConsumer interface {
	GetId() uint32
	IsNavigationComponent() bool
	Tapped(*fyne.PointEvent)
	TappedSecondary(*fyne.PointEvent)
	Visible() bool
	// DoubleTapped(*fyne.PointEvent)
}

type RouterTappedConsumer interface {
	IsNavigationComponent() bool
	Tapped(*TiniPointEvent)
	TappedSecondary(*TiniPointEvent)
	Visible() bool
	// DoubleTapped(*fyne.PointEvent)
}

type MouseConsumer interface {
	//	MouseDown(me *desktop.MouseEvent)
}

type DraggedConsumer interface {
	IsDragging() bool
	Dragged(d *fyne.DragEvent)
	DragEnd()
	SetGesture(gesture GestureType)
}

// EventConsumer describes an event consumer
type EventRouter interface {
	ContentConsumer
	RouterTappedConsumer
	MouseConsumer
	DraggedConsumer
	AddConsumers(...interface{})
}

type RouterConsumer interface {
	SetRouter(er EventRouter)
}

// EventRouterBase -- base class.. common destinations for routing events.
type EventRouterBase struct {
	TapConsumers     []TappedConsumer
	DragConsumers    []DraggedConsumer
	ContentConsumers []ContentConsumer
	MouseConsumers   []MouseConsumer
}
