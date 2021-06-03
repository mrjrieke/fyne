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
	TickerContent      string = "TickerContent"
	SearchQueryContent string = "SearchQueryContent"
	PageContent        string = "PageContent"
)

type TagType int

type TextTag struct {
	StartIndex int
	EndIndex   int // Optional -- some tag values are in the text already like CharacterQuote
	TagValue   string
	TagType    TagType
}

type TextStack struct {
	Body     strings.Builder
	TextTags []TextTag
}

// ContentEvent defines the parameters of a content event
type ContentEvent struct {
	ContentAction string
	ContentType   string
	Content       *TextStack
}

type ContentConsumer interface {
	ContentChanged(*ContentEvent)
	GetSelected(*fyne.PointEvent) string
}

type TappedConsumer interface {
	IsNavigationComponent() bool
	Tapped(*fyne.PointEvent)
	TappedSecondary(*fyne.PointEvent)
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
	TappedConsumer
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
