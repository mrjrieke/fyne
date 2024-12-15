package dialog

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/internal/widget"
	"fyne.io/fyne/v2/theme"
	commonwidget "fyne.io/fyne/v2/widget"
)

//import "github.com/davecgh/go-spew/spew"

var _ fyne.Draggable = (*TickerPopUp)(nil)
var _ ContentConsumer = (*TickerPopUp)(nil)

type ScrollMode uint8

var (
	ScrollNone               = ScrollMode(0)
	ScrollLeft               = ScrollMode(1)
	ScrollRight              = ScrollMode(2)
	ScrollFreeLeftBound      = ScrollMode(3)
	ScrollFreeRightBound     = ScrollMode(4)
	ScrollFreeLeftRightBound = ScrollMode(5)
	ScrollFree               = ScrollMode(6)
)

type DragState uint8

var (
	DragInterrupted = DragState(0)
	DragFinished    = DragState(1)
	DragEnded       = DragState(2)
	DragStarted     = DragState(3)
	DragEmpty       = DragState(4)
)

type GestureType int

const (
	GestureNone GestureType = iota
	// No gesture accumulator
	GestureNoneAccumulate = iota + 1
	// Pull gesture
	GesturePull = iota + 2
	// Push gesture
	GesturePush = iota + 3
)

type DragStyle bool

var (
	DragNatural = DragStyle(true)
	DragAnti    = DragStyle(false)
)

type TextWidget interface {
	GetText() string
	SetText(string)
	SetEdit(bool)
	SetSelected(string)
	TextStyle() fyne.TextStyle
}

type ringBuffer struct {
	lock           sync.RWMutex
	ScrollLock     bool // Temporarily lock scroll due to content.
	ScrollMode     ScrollMode
	offsetAccum    float32 // Accumulator for character offsets.
	data           []rune
	start          int // Start of ringBuffer can be anywhere in array.
	bufferWidth    int
	width          float32        // Width of draw area
	SeparatorRune  rune           // Separator character
	entryPadding   float32        // Any padding around entries
	labelFontSize  float32        // Font size in label
	labelTextStyle fyne.TextStyle // font text style
	dragStyle      DragStyle      // Drag or Anti-drag  true == with turn.
}

var charSizeCache = map[string]*fyne.Size{}

type PopupTickerListener interface {
	TapCallback(fyne.Tappable, *fyne.PointEvent)
	TouchCallback(fyne.Tappable, *mobile.TouchEvent)
}

func (rb *ringBuffer) Init(start int, data []rune) {
	rb.data = data
	rb.start = start
}

// Turn - rotates the ringbuffer by appropriate offset.  -offset is left, +offset is right.
func (rb *ringBuffer) Turn(offsetInput int) (bool, int) {
	if len(rb.data) == 0 || rb.ScrollMode == ScrollNone || rb.ScrollLock {
		return false, 0
	}

	// Accumulate offsetInput and convert to an actual offset.
	offset := 0

	rb.offsetAccum = rb.offsetAccum + float32(offsetInput)

	if rb.offsetAccum < 0 {
		left := rb.start - 1
		if rb.start == 0 {
			left = len(rb.data) - 1
		}
		leftChar := rb.MeasureText(string(rb.data[left]), rb.labelFontSize, rb.labelTextStyle)
		if rb.offsetAccum <= -leftChar.Width {
			offset = int(rb.offsetAccum)
			rb.offsetAccum = rb.offsetAccum + leftChar.Width
		}
	} else if rb.offsetAccum > 0 {
		right := rb.start + 1
		if rb.start >= len(rb.data)-1 {
			right = 0
		}
		rightChar := rb.MeasureText(string(rb.data[right]), rb.labelFontSize, rb.labelTextStyle)
		if rb.offsetAccum >= rightChar.Width {
			offset = int(rb.offsetAccum)
			rb.offsetAccum = rb.offsetAccum - rightChar.Width
		}
	}

	if offset > 0 {
		offset = 1
	} else if offset < 0 {
		offset = -1
	} else {
		return false, 0
	}

	var start int
	direction := offset

	if rb.dragStyle == DragNatural {
		start = rb.start - offset
	} else {
		direction = -offset
		start = rb.start + offset
	}

	switch rb.ScrollMode {
	case ScrollLeft:
		if direction > 0 {
			return false, 0
		}
		break
	case ScrollRight:
		if direction < 0 {
			return false, 0
		}
		break
	case ScrollFreeLeftRightBound:
		if start < 0 {
			start = 0
		} else {
			currentLen := len(rb.Data(false))
			if (start + currentLen) > len(rb.data) {
				start = len(rb.data) - currentLen - 1
			}
		}
		break
	case ScrollFreeLeftBound:
		if start < 0 {
			start = 0
		}
		break
	case ScrollFreeRightBound:
		if start > len(rb.data) {
			start = len(rb.data) - 1
		}
		break
	case ScrollFree:
		break
	default:
	}
	if start == rb.start {
		return false, 0
	}
	rb.start = start

	if rb.start > len(rb.data) {
		rb.start = rb.start % len(rb.data)
	} else if rb.start < 0 {
		rb.start = len(rb.data) + rb.start
	}
	return true, offset
}

func (rb *ringBuffer) MeasureText(text string, size float32, style fyne.TextStyle) *fyne.Size {
	rb.lock.Lock()
	defer rb.lock.Unlock()
	if charSize, ok := charSizeCache[text]; ok {
		if charSize.Width > 0 {
			return charSize
		}
	}
	charSize := fyne.MeasureText(text, size, style)
	charSizeCache[text] = &charSize
	return &charSize
}

// GetSelected -- given pixel offset, returns selected text.
func (rb *ringBuffer) GetSelected(popupTickerPosX float32, selectedPosX float32) string {
	currentData := rb.Data(false)
	if len(currentData) == 0 {
		return ""
	}

	// Seek the offset by character widths.
	width := popupTickerPosX
	nearestIndex := 0
	hasSeparatorRune := false
	for i := 0; i < len(currentData); i++ {
		if currentData[i] == rb.SeparatorRune {
			width = width + rb.entryPadding
			hasSeparatorRune = true
		}
		charSize := rb.MeasureText(string(currentData[i]), rb.labelFontSize, rb.labelTextStyle)
		width = width + (*charSize).Width
		if width > selectedPosX {
			for j := i + 1; !hasSeparatorRune && j < len(currentData); j++ {
				if currentData[j] == rb.SeparatorRune {
					hasSeparatorRune = true
				}
			}
			break
		} else {
			nearestIndex = i
		}
		if currentData[i] == rb.SeparatorRune {
			// Do not add entry padding here.  It is not needed for calculating selection.
			hasSeparatorRune = true
		}
	}
	if !hasSeparatorRune {
		return string(currentData)
	}

	if currentData[nearestIndex] == rb.SeparatorRune {
		// Don't start on a separator.
		nearestIndex = nearestIndex + 1
	}

	nearestSeparatorFound := false
	for i := nearestIndex; i >= 0; i-- {
		if currentData[i] == rb.SeparatorRune {
			nearestSeparatorFound = true
			break
		}
		nearestIndex = i
	}

	endIndex := nearestIndex
	farthestSeparatorFound := false

	for i := nearestIndex; i < len(currentData); i++ {
		if currentData[i] == rb.SeparatorRune {
			farthestSeparatorFound = true
			endIndex = i
			break
		}
	}
	var result string

	if !nearestSeparatorFound {
		fullData := rb.Data(true)

		for i := len(fullData) - 1; i >= 0; i-- {
			if fullData[i] == rb.SeparatorRune {
				nearestSeparatorFound = true
				break
			}
			nearestIndex = i
		}
		if nearestIndex > endIndex {
			result = string(fullData[nearestIndex:]) + string(fullData[0:endIndex])
		} else {
			result = string(rb.data[nearestIndex:endIndex])
		}
	} else if !farthestSeparatorFound {
		fullData := rb.Data(true)

		for i := nearestIndex; i < len(fullData); i++ {
			if fullData[i] == rb.SeparatorRune {
				farthestSeparatorFound = true
				endIndex = i
				break
			}
		}
		if !farthestSeparatorFound {
			result = string(fullData[nearestIndex:])
			for i := 0; i < len(fullData); i++ {
				if fullData[i] == rb.SeparatorRune {
					farthestSeparatorFound = true
					endIndex = i
					break
				}
			}
			result = result + string(fullData[0:endIndex])
		} else {
			result = string(fullData[nearestIndex:endIndex])
		}
	} else {
		result = string(currentData[nearestIndex:endIndex])
	}

	return result
}

// Data - returns current data at current turn, read circularly
func (rb *ringBuffer) Data(complete bool) []rune {
	var data []rune
	if rb.start == 0 {
		data = rb.data
	} else {
		data = append(rb.data[rb.start:], rb.data[0:rb.start]...)
	}

	if !complete && rb.width > 0 {
		width := float32(0)
		boundIndex := 0
		for i := 0; i < len(data); i++ {
			if data[i] == rb.SeparatorRune {
				width = width + rb.entryPadding
			}
			charSize := rb.MeasureText(string(data[i]), rb.labelFontSize, rb.labelTextStyle)
			width = width + (*charSize).Width
			if width > rb.width {
				break
			} else {
				boundIndex = i
			}
			if data[i] == rb.SeparatorRune {
				width = width + rb.entryPadding
			}
		}

		if boundIndex == len(data)-1 {
			rb.ScrollLock = true
		} else {
			rb.ScrollLock = false
		}
		return data[0:boundIndex]
	} else {
		return data
	}
}

func (rb *ringBuffer) Length() int {
	return len(rb.data)
}

// TickerPopUp is a widget that can float above the user interface.
// It wraps any standard elements with padding and a shadow.
// If it is modal then the shadow will cover the entire canvas it hovers over and block interactions.
type TickerPopUp struct {
	commonwidget.BaseWidget

	Content fyne.CanvasObject
	Canvas  fyne.Canvas

	Id               uint32
	RouteProvider    func(string) []uint32
	CurrentSelection string

	// EventRouter is a router of touch events.
	EventRouter EventRouter

	popupTickerListener PopupTickerListener
	rb                  ringBuffer // backing the content with a ringBuffer.
	innerPos            fyne.Position
	innerSize           fyne.Size
	modal               bool
	overlayShown        bool
	draggedX            *fyne.DragEvent
	draggedXPrev        *fyne.DragEvent
	draggedTime         time.Time
	draggedTimePrev     time.Time
	animate             bool
	dragging            DragState
	interruptSlide      chan bool
	dragScale           int
	dsCount             int
}

func (p *TickerPopUp) GetId() uint32 {
	return p.Id
}

func (p *TickerPopUp) SetRouter(er EventRouter) {
	p.EventRouter = er
}

func (p *TickerPopUp) DragClear() {
	p.dragging = DragInterrupted
}

// Hide this widget, if it was previously visible
func (p *TickerPopUp) Hide() {
	p.DragClear()
	if p.overlayShown {
		p.Canvas.Overlays().Remove(p)
		p.overlayShown = false
	}
	p.BaseWidget.Hide()
}

// Move the widget to a new position. A TickerPopUp position is absolute to the top, left of its canvas.
// For TickerPopUp this actually moves the content so checking Position() will not return the same value as is set here.
func (p *TickerPopUp) Move(pos fyne.Position) {
	if p.modal {
		return
	}
	if pos.X != 0 && pos.Y != 0 {
		p.innerPos = pos
	}
	p.Refresh()
}

// Resize changes the size of the TickerPopUp.
// TickerPopUps always have the size of their canvas.
// However, Resize changes the size of the TickerPopUp's content.
//
// Implements: fyne.Widget
func (p *TickerPopUp) Resize(size fyne.Size) {
	if p.innerSize.Width == size.Width && p.innerSize.Height == size.Height {
		if p.BaseWidget.Size().Width == p.Canvas.Size().Width && p.BaseWidget.Size().Height == p.Canvas.Size().Height {
			// Don't do anything if already sized correctly
			return
		}
	}
	p.innerSize = size
	p.BaseWidget.Resize(size)
	// The canvas size might not have changed and therefore the Resize won't trigger a layout.
	// Until we have a widget.Relayout() or similar, the renderer's refresh will do the re-layout.
	p.Refresh()
}

// Show this pop-up as overlay if not already shown.
func (p *TickerPopUp) Show() {
	if !p.overlayShown {
		if p.Size().IsZero() {
			p.Resize(p.MinSize())
		}
		p.Canvas.Overlays().Add(p)
		p.overlayShown = true
	}
	p.BaseWidget.Show()
}

// ShowAtPosition shows this pop-up at the given position.
func (p *TickerPopUp) ShowAtPosition(pos fyne.Position) {
	p.Move(pos)
	p.Show()
}

// DragEnd function.
func (p *TickerPopUp) DragEnd() {
	if !p.Visible() {
		// Handled hints outside of our bounds
		return
	}

	if p.dragging <= DragEnded || !p.animate || p.draggedX == nil || p.draggedXPrev == nil {
		return
	}
	p.dragging = DragEnded

	// Calculate distance moved at end of a drag.
	d := p.draggedX.Position.X - p.draggedXPrev.Position.X
	posX := p.draggedX.AbsolutePosition.X
	prevPosX := p.draggedXPrev.AbsolutePosition.X
	if posX == 0 && prevPosX == 0 {
		posX = p.draggedX.Position.X
		prevPosX = p.draggedXPrev.Position.X
	}

	da := posX - prevPosX
	if da == 0 {
		return
	}

	initial := time.Duration(16640295) // .016 seconds
	elapsed := initial

	go func() {
		// Generate events... logarithmically.
		// distance == distance moved between last 2 points.
		// time = time spent to move from those 2 last code points.
		interrupt := false
		i := float32(0)

		go func() {
			slideTimeout := make(chan bool, 1)

			go func() {
				time.Sleep(2 * time.Second)
				slideTimeout <- true
			}()

			select {
			case <-p.interruptSlide:
				interrupt = true
				break
			case <-slideTimeout:
				interrupt = true
				break
			}
			p.dragging = DragFinished
		}()

		var slideTime time.Duration = 0
		for {
			if p.dragging == DragStarted || p.dragging == DragInterrupted || slideTime > 1500000000 {
				break
			}

			elapsed = elapsed + (elapsed - time.Duration(float64(initial)/math.Exp(2*float64(elapsed)/1000000000)))

			slideTime = slideTime + (elapsed * time.Nanosecond)
			time.Sleep(elapsed * time.Nanosecond)

			e := &fyne.DragEvent{
				PointEvent: fyne.PointEvent{Position: fyne.NewPos(p.draggedX.Position.X+d, p.draggedX.Position.Y),
					AbsolutePosition: fyne.NewPos(p.draggedX.AbsolutePosition.X+da, p.draggedX.AbsolutePosition.Y)},
				Dragged: fyne.Delta{DX: d, DY: 0},
			}
			if interrupt {
				break
			}

			p.DraggedHelper(e)

			i = i + d
		}

	}()

}

func (p *TickerPopUp) IsNavigationComponent() bool {
	return true
}

func (p *TickerPopUp) Visible() bool {
	return p.BaseWidget.Visible()
}

// Tapped is called when the user taps the tickerPopUp background - if not modal then dismiss this widget
func (p *TickerPopUp) Tapped(e *fyne.PointEvent) {
	if e.AbsolutePosition.X < p.innerPos.X || e.AbsolutePosition.Y < p.innerPos.Y || e.AbsolutePosition.X > (p.innerPos.X+p.innerSize.Width) || e.AbsolutePosition.Y > (p.innerPos.Y+p.innerSize.Height) {
		p.CurrentSelection = ""
		if !p.EventRouter.IsDragging() {
			if p.Visible() {
				p.Hide()
			} else {
				p.Show()
			}
		}
		return
	}
	if p.dragging != DragInterrupted && p.dragging != DragFinished {
		p.dragging = DragFinished
		return
	}

	if p.popupTickerListener != nil {
		p.GetSelectedByPosition(&e.AbsolutePosition)
		p.popupTickerListener.TapCallback(p, e)
	}
}

func (p *TickerPopUp) DoubleTapped(e *fyne.PointEvent) {
}

// TouchDown is called when this entry gets a touch down event on mobile device, we ensure we have focus.
//
// Since: 2.1
//
// Implements: mobile.Touchable
func (p *TickerPopUp) TouchDown(e *mobile.TouchEvent) {
	// now := time.Now().UnixMilli()
	// if !p.Disabled() {
	// 	e.requestFocus()
	// }
	// if e.isTripleTap(now) {
	// 	e.selectCurrentRow()
	// 	return
	// }

	// if e.AbsolutePosition.X < p.innerPos.X || e.AbsolutePosition.Y < p.innerPos.Y || e.AbsolutePosition.X > (p.innerPos.X+p.innerSize.Width) || e.AbsolutePosition.Y > (p.innerPos.Y+p.innerSize.Height) {
	// 	p.CurrentSelection = ""
	// 	if !p.EventRouter.IsDragging() {
	// 		if p.Visible() {
	// 			p.Hide()
	// 		} else {
	// 			p.Show()
	// 		}
	// 	}
	// 	return
	// }
	// if p.dragging != DragInterrupted && p.dragging != DragFinished {
	// 	p.dragging = DragFinished
	// 	return
	// }

	// if p.popupTickerListener != nil {
	// 	p.GetSelectedByPosition(&e.AbsolutePosition)
	// 	p.popupTickerListener.TouchCallback(p, e)
	// }
}

// TouchUp is called when this entry gets a touch up event on mobile device.
//
// Since: 2.1
//
// Implements: mobile.Touchable
func (p *TickerPopUp) TouchUp(e *mobile.TouchEvent) {
	// if e.AbsolutePosition.X < p.innerPos.X || e.AbsolutePosition.Y < p.innerPos.Y || e.AbsolutePosition.X > (p.innerPos.X+p.innerSize.Width) || e.AbsolutePosition.Y > (p.innerPos.Y+p.innerSize.Height) {
	// 	p.CurrentSelection = ""
	// 	if !p.EventRouter.IsDragging() {
	// 		if p.Visible() {
	// 			p.Hide()
	// 		} else {
	// 			p.Show()
	// 		}
	// 	}
	// 	return
	// }
	// if p.dragging != DragInterrupted && p.dragging != DragFinished {
	// 	p.dragging = DragFinished
	// 	return
	// }

	// if p.popupTickerListener != nil {
	// 	p.GetSelectedByPosition(&e.AbsolutePosition)
	// 	p.popupTickerListener.TouchCallback(p, e)
	// }
}

// TouchCancel is called when this entry gets a touch cancel event on mobile device (app was removed from focus).
//
// Since: 2.1
//
// Implements: mobile.Touchable
func (p *TickerPopUp) TouchCancel(*mobile.TouchEvent) {
}

func (p *TickerPopUp) endOffset() float32 {
	return p.innerPos.X + theme.Padding()
}

func (p *TickerPopUp) getRatio(pos *fyne.Position) float64 {
	x := pos.X - p.innerPos.X

	tickerWidth := p.rb.width

	if x > p.innerPos.X+tickerWidth {
		return 1.0
	} else if pos.X < p.innerPos.X {
		return 0.0
	} else {
		return float64(x) / float64(tickerWidth-(2*theme.Padding()))
	}
}

func (p *TickerPopUp) ContentChanged(ce *ContentEvent) {
	if ce.ContentType == TickerContent {
		if ce.ContentAction == RefreshContent {
			p.SetText([]rune(ce.Content.Body.String()))
			p.SetEdit(false)
			// TODO: Seems innefficient here.
			p.Resize(p.innerSize)
			//
			p.Content.Refresh()
			// p.Refresh()
		} else if ce.ContentAction == CreateContentRequest {
			// p.SetText([]rune(ce.Content))
			if ce.Content.Body.Len() > 0 {
				// TODO: Output String() here to see if it is actually updating ringBuffer to what we want!
				p.SetSearchText([]rune(ce.Content.Body.String()))
			}
			p.Resize(p.innerSize)
			p.Refresh()
		}
	}
}

func (p *TickerPopUp) GetSelected(e *fyne.PointEvent) string {
	selection := p.GetSelectedByPosition(&e.AbsolutePosition)
	// Notify router of a selection
	if selection != "" {
		contentEvent := ContentEvent{
			SourceWidgetId:       p.Id,
			DestinationWidgetIds: p.RouteProvider(ItemSelectedRequest),
			ContentAction:        ItemSelectedRequest,
			ContentType:          TickerContent,
			Content:              &TextStack{},
		}
		contentEvent.Content.Body.WriteString(selection)
		p.EventRouter.ContentChanged(&contentEvent)
	}
	return selection
}

func (p *TickerPopUp) GetSelectedByPosition(absolutePos *fyne.Position) string {
	if absolutePos.X < p.innerPos.X || absolutePos.Y < p.innerPos.Y || absolutePos.X > (p.innerPos.X+p.innerSize.Width) || absolutePos.Y > (p.innerPos.Y+p.innerSize.Height) {
		p.CurrentSelection = ""
		return ""
	}
	if p.IsDragging() {
		return ""
	}
	p.CurrentSelection = p.rb.GetSelected(p.Content.Position().X+theme.Padding(), absolutePos.X)
	// Update fact that content is about to change, so refresh selection stack.
	contentEvent := ContentEvent{
		SourceWidgetId: p.Id,
		ContentAction:  RefreshTickerContent,
		ContentType:    TickerContent,
		Content:        &TextStack{},
	}
	contentRunes := p.rb.Data(true)
	for i := 0; i < len(contentRunes); i++ {
		contentEvent.Content.Body.WriteRune(contentRunes[i])
	}

	p.EventRouter.ContentChanged(&contentEvent)

	return p.CurrentSelection
}
func (p *TickerPopUp) SetSearchText(data []rune) {
	p.rb.data = data
	p.rb.start = 0 // Re-init start after content change
}

func (p *TickerPopUp) SetText(data []rune) {
	p.rb.data = data
	p.rb.start = 0 // Re-init start after content change
	switch p.Content.(type) {
	case *commonwidget.Label:
		p.Content.(*commonwidget.Label).Text = string(p.rb.Data(false))
		break
	case *commonwidget.Entry:
		p.Content.(*commonwidget.Entry).Text = string(p.rb.Data(false))
		break
	case TextWidget:
		p.Content.(TextWidget).SetText(string(p.rb.Data(false)))
	}
}

func (p *TickerPopUp) SetEdit(edit bool) {
	switch p.Content.(type) {
	case *commonwidget.Label:
		break
	case *commonwidget.Entry:
		break
	case TextWidget:
		p.Content.(TextWidget).SetEdit(edit)
	}
}

func (p *TickerPopUp) IsDragging() bool {
	//return (p.dragging != DragInterrupted)

	return (p.dragging != DragInterrupted && p.dragging != DragFinished && p.dragging != DragEmpty)
}

func (p *TickerPopUp) SetGesture(gesture GestureType) {

}

func (p *TickerPopUp) DraggedHelper(e *fyne.DragEvent) int {
	if p.draggedX == nil {
		p.draggedX = e
		p.draggedTime = time.Now()
		return 0
	}
	posX := e.AbsolutePosition.X
	posY := e.AbsolutePosition.Y
	if posX == 0 && posY == 0 {
		posX = e.Position.X
		posY = e.Position.Y
	}

	if (posX < p.innerPos.X) || posY < p.innerPos.Y || (posX > (p.innerPos.X + p.innerSize.Width)) || posY > (p.innerPos.Y+p.innerSize.Height) {
		if p.dragging == DragInterrupted {
			p.Hide()
		}
		return 0
	}

	var diffPosition fyne.Position
	diffPosition.X = e.Position.X - p.draggedX.Position.X
	diffPosition.Y = e.Position.Y - p.draggedX.Position.Y

	p.draggedXPrev = p.draggedX
	p.draggedX = e
	p.draggedTimePrev = p.draggedTime
	p.draggedTime = time.Now()
	diffX := int(diffPosition.X)

	if diffX <= p.dragScale && diffX >= -p.dragScale {
		diffX = diffX % p.dragScale
	} else {
		diffX = 0
	}

	if diffX != 0 {
		turned, diffX := p.rb.Turn(int(diffX))
		if !turned {
			return diffX
		}

		switch p.Content.(type) {
		case *commonwidget.Label:
			p.Content.(*commonwidget.Label).Text = string(p.rb.Data(false))
			p.Content.Refresh()
			break
		case *commonwidget.Entry:
			p.Content.(*commonwidget.Entry).Text = string(p.rb.Data(false))
			p.Content.Refresh()
			break
		case TextWidget:
			p.Content.(TextWidget).SetText(string(p.rb.Data(false)))
		}
		return diffX
	}
	return 0
}

func (p *TickerPopUp) Dragged(e *fyne.DragEvent) {
	posX := e.AbsolutePosition.X
	posY := e.AbsolutePosition.Y
	if posX == 0 && posY == 0 {
		posX = e.Position.X
		posY = e.Position.Y
	}
	if !p.Visible() ||
		posX < p.innerPos.X ||
		posY < p.innerPos.Y ||
		posX > (p.innerPos.X+p.innerSize.Width) ||
		posY > (p.innerPos.Y+p.innerSize.Height) {
		// Handled hints outside of our bounds
		return
	}

	select {
	case p.interruptSlide <- true:
		break
	default:
		break
	}

	p.dragging = DragStarted
	p.DraggedHelper(e)
}

// TappedSecondary is called when the user right/alt taps the background - if not modal then dismiss this widget
func (p *TickerPopUp) TappedSecondary(e *fyne.PointEvent) {
	if e.AbsolutePosition.X < p.innerPos.X || e.AbsolutePosition.Y < p.innerPos.Y || e.AbsolutePosition.X > (p.innerPos.X+p.innerSize.Width) || e.AbsolutePosition.Y > (p.innerPos.Y+p.innerSize.Height) {
		p.CurrentSelection = ""
		if !p.EventRouter.IsDragging() {
			if p.Visible() {
				//			p.Hide()
			} else {
				p.Show()
			}
		}
		return
	}
	if p.dragging != DragInterrupted && p.dragging != DragFinished {
		p.dragging = DragFinished
		return
	}

	if p.popupTickerListener != nil {
		p.GetSelectedByPosition(&e.AbsolutePosition)
		p.popupTickerListener.TapCallback(p, e)
	}
	if !p.modal {
		//		p.Hide()
	}
}

// MinSize returns the size that this widget should not shrink below
func (p *TickerPopUp) MinSize() fyne.Size {
	p.ExtendBaseWidget(p)
	return p.innerSize
}

// CreateRenderer is a private method to Fyne which links this widget to its renderer
func (p *TickerPopUp) CreateRenderer() fyne.WidgetRenderer {
	p.ExtendBaseWidget(p)
	bg := canvas.NewRectangle(theme.BackgroundColor())
	objects := []fyne.CanvasObject{bg, p.Content}
	if p.modal {
		return &modalTickerPopUpRenderer{
			widget.NewShadowingRenderer(objects, widget.DialogLevel),
			tickerPopupBaseRenderer{tickerPopUp: p, bg: bg},
		}
	}

	return &tickerPopUpRenderer{
		widget.NewShadowingRenderer(objects, widget.PopUpLevel),
		tickerPopupBaseRenderer{tickerPopUp: p, bg: bg},
	}
}
func (p *TickerPopUp) GetSeparatorRune() rune {
	return p.rb.SeparatorRune
}

func (p *TickerPopUp) Animate() {
	p.animate = true
}

// NewTickerPopUpAtPosition creates a new tickerPopUp for the specified content at the specified absolute position.
// It will then display the popup on the passed canvas.
//
// Deprecated: Use ShowTickerPopUpAtPosition() instead.
func NewTickerPopUpAtPosition(scrollMode ScrollMode, content fyne.CanvasObject, canvas fyne.Canvas, popupTickerListener PopupTickerListener, pos fyne.Position, size fyne.Size, fontSize float32, separator rune, entryPadding float32, dragStyle DragStyle, routeProvider func(string) []uint32) *TickerPopUp {
	p := newTickerPopUp(scrollMode, content, canvas, popupTickerListener, size, fontSize, separator, entryPadding, dragStyle, routeProvider)
	p.ShowAtPosition(pos)
	return p
}

// ShowTickerPopUpAtPosition creates a new tickerPopUp for the specified content at the specified absolute position.
// It will then display the popup on the passed canvas.
func ShowTickerPopUpAtPosition(scrollMode ScrollMode, content fyne.CanvasObject, canvas fyne.Canvas, pos fyne.Position, popupTickerListener PopupTickerListener, size fyne.Size, fontSize float32, separator rune, entryPadding float32, dragStyle DragStyle, routeProvider func(string) []uint32) {
	newTickerPopUp(scrollMode, content, canvas, popupTickerListener, size, fontSize, separator, entryPadding, dragStyle, routeProvider).ShowAtPosition(pos)
}

func newTickerPopUp(scrollMode ScrollMode, content fyne.CanvasObject, canvas fyne.Canvas, popupTickerListener PopupTickerListener, size fyne.Size, fontSize float32, separator rune, entryPadding float32, dragStyle DragStyle, routeProvider func(string) []uint32) *TickerPopUp {
	rb := ringBuffer{ScrollMode: scrollMode, start: 0, labelFontSize: fontSize, width: size.Width, dragStyle: dragStyle, SeparatorRune: separator, entryPadding: entryPadding}

	// TODO: would be nice if Label and Entry implemented GetText() and TextStyle().  Then remove switch and access directly.
	switch content.(type) {
	case *commonwidget.Label:
		rb.data = []rune(content.(*commonwidget.Label).Text)
		rb.labelTextStyle = content.(*commonwidget.Label).TextStyle
		content.(*commonwidget.Label).Text = string(rb.Data(false))
		break
	case *commonwidget.Entry:
		rb.data = []rune(content.(*commonwidget.Entry).Text)
		rb.labelTextStyle = fyne.TextStyle{} // Entry should provide.
		content.(*commonwidget.Entry).Text = string(rb.Data(false))
		break
	case TextWidget:
		rb.data = []rune(content.(TextWidget).GetText())
		rb.labelTextStyle = content.(TextWidget).TextStyle()
		content.(TextWidget).SetText(string(rb.Data(false)))
	}

	ret := &TickerPopUp{Content: content, rb: rb, Canvas: canvas, popupTickerListener: popupTickerListener, modal: false, dragScale: 100, interruptSlide: make(chan bool), RouteProvider: routeProvider}
	ret.ExtendBaseWidget(ret)
	ret.Resize(size)
	return ret
}

// NewTickerPopUp creates a new tickerPopUp for the specified content and displays it on the passed canvas.
//
// Deprecated: This will no longer show the pop-up in 2.0. Use ShowTickerPopUp() instead.
func NewTickerPopUp(scrollMode ScrollMode, content fyne.CanvasObject, canvas fyne.Canvas, popupTickerListener PopupTickerListener, size fyne.Size, fontSize float32, separator rune, entryPadding float32, dragStyle DragStyle, routeProvider func(string) []uint32) *TickerPopUp {
	return NewTickerPopUpAtPosition(scrollMode, content, canvas, popupTickerListener, fyne.NewPos(0, 0), size, fontSize, separator, entryPadding, dragStyle, routeProvider)
}

// ShowTickerPopUp creates a new tickerPopUp for the specified content and displays it on the passed canvas.
func ShowTickerPopUp(scrollMode ScrollMode, content fyne.CanvasObject, canvas fyne.Canvas, popupTickerListener PopupTickerListener, size fyne.Size, fontSize float32, separator rune, entryPadding float32, dragStyle DragStyle, routeProvider func(string) []uint32) {
	newTickerPopUp(scrollMode, content, canvas, popupTickerListener, size, fontSize, separator, entryPadding, dragStyle, routeProvider).Show()
}

func newModalTickerPopUp(content fyne.CanvasObject, canvas fyne.Canvas) *TickerPopUp {
	p := &TickerPopUp{Content: content, Canvas: canvas, modal: true}
	p.ExtendBaseWidget(p)
	return p
}

// NewModalTickerPopUp creates a new tickerPopUp for the specified content and displays it on the passed canvas.
// A modal TickerPopUp blocks interactions with underlying elements, covered with a semi-transparent overlay.
//
// Deprecated: This will no longer show the pop-up in 2.0. Use ShowModalTickerPopUp instead.
func NewModalTickerPopUp(content fyne.CanvasObject, canvas fyne.Canvas) *TickerPopUp {
	p := newModalTickerPopUp(content, canvas)
	p.Show()
	return p
}

// ShowModalTickerPopUp creates a new tickerPopUp for the specified content and displays it on the passed canvas.
// A modal TickerPopUp blocks interactions with underlying elements, covered with a semi-transparent overlay.
func ShowModalTickerPopUp(content fyne.CanvasObject, canvas fyne.Canvas) {
	p := newModalTickerPopUp(content, canvas)
	p.Show()
}

type tickerPopupBaseRenderer struct {
	tickerPopUp *TickerPopUp
	bg          *canvas.Rectangle
}

func (r *tickerPopupBaseRenderer) padding() fyne.Size {
	return fyne.NewSize(theme.Padding()*2, theme.Padding()*2)
}

func (r *tickerPopupBaseRenderer) offset() fyne.Position {
	return fyne.NewPos(theme.Padding(), theme.Padding())
}

type tickerPopUpRenderer struct {
	*widget.ShadowingRenderer
	tickerPopupBaseRenderer
}

func (r *tickerPopUpRenderer) Layout(_ fyne.Size) {
	r.tickerPopUp.Content.Resize(r.tickerPopUp.innerSize.Subtract(r.padding()))

	innerPos := r.tickerPopUp.innerPos
	if innerPos.X+r.tickerPopUp.innerSize.Width > r.tickerPopUp.Canvas.Size().Width {
		innerPos.X = r.tickerPopUp.Canvas.Size().Width - r.tickerPopUp.innerSize.Width
		if innerPos.X < 0 {
			innerPos.X = 0
		}
	}
	if innerPos.Y+r.tickerPopUp.innerSize.Height > r.tickerPopUp.Canvas.Size().Height {
		innerPos.Y = r.tickerPopUp.Canvas.Size().Height - r.tickerPopUp.innerSize.Height
		if innerPos.Y < 0 {
			innerPos.Y = 0
		}
	}
	r.tickerPopUp.Content.Move(innerPos.Add(r.offset()))

	r.bg.Resize(r.tickerPopUp.innerSize)
	r.bg.Move(innerPos)
	r.LayoutShadow(r.tickerPopUp.innerSize, innerPos)
}

func (r *tickerPopUpRenderer) MinSize() fyne.Size {
	return r.tickerPopUp.Content.MinSize().Add(r.padding())
}

func (r *tickerPopUpRenderer) Refresh() {
	r.bg.FillColor = theme.BackgroundColor()
	if r.bg.Size() != r.tickerPopUp.innerSize || r.bg.Position() != r.tickerPopUp.innerPos {
		r.Layout(r.tickerPopUp.Size())
	}
}

func (r *tickerPopUpRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

type modalTickerPopUpRenderer struct {
	*widget.ShadowingRenderer
	tickerPopupBaseRenderer
}

func (r *modalTickerPopUpRenderer) Layout(canvasSize fyne.Size) {
	padding := r.padding()
	requestedSize := r.tickerPopUp.innerSize.Subtract(padding)
	size := r.tickerPopUp.Content.MinSize().Max(requestedSize)
	size = size.Min(canvasSize.Subtract(padding))
	pos := fyne.NewPos((canvasSize.Width-size.Width)/2, (canvasSize.Height-size.Height)/2)
	r.tickerPopUp.Content.Move(pos)
	r.tickerPopUp.Content.Resize(size)

	innerPos := pos.Subtract(r.offset())
	r.bg.Move(innerPos)
	r.bg.Resize(size.Add(padding))
	r.LayoutShadow(r.tickerPopUp.innerSize, innerPos)
}

func (r *modalTickerPopUpRenderer) MinSize() fyne.Size {
	return r.tickerPopUp.Content.MinSize().Add(r.padding())
}

func (r *modalTickerPopUpRenderer) Refresh() {
	r.bg.FillColor = theme.BackgroundColor()
	if r.bg.Size() != r.tickerPopUp.innerSize {
		r.Layout(r.tickerPopUp.Size())
	}
}

func (r *modalTickerPopUpRenderer) BackgroundColor() color.Color {
	return theme.ShadowColor()
}
