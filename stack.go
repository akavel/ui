// 13 february 2014

package ui

import (
	"fmt"
	"sync"
)

type orientation bool

const (
	horizontal orientation = false
	vertical   orientation = true
)

// A Stack stacks controls horizontally or vertically within the Stack's parent.
// A horizontal Stack gives all controls the same height and their preferred widths.
// A vertical Stack gives all controls the same width and their preferred heights.
// Any extra space at the end of a Stack is left blank.
// Some controls may be marked as "stretchy": when the Window they are in changes size, stretchy controls resize to take up the remaining space after non-stretchy controls are laid out. If multiple controls are marked stretchy, they are alloted equal distribution of the remaining space.
type Stack struct {
	lock          sync.Mutex
	created       bool
	orientation   orientation
	controls      []Control
	stretchy      []bool
	width, height []int // caches to avoid reallocating these each time
}

func newStack(o orientation, controls ...Control) *Stack {
	return &Stack{
		orientation: o,
		controls:    controls,
		stretchy:    make([]bool, len(controls)),
		width:       make([]int, len(controls)),
		height:      make([]int, len(controls)),
	}
}

// NewHorizontalStack creates a new Stack that arranges the given Controls horizontally.
func NewHorizontalStack(controls ...Control) *Stack {
	return newStack(horizontal, controls...)
}

// NewVerticalStack creates a new Stack that arranges the given Controls vertically.
func NewVerticalStack(controls ...Control) *Stack {
	return newStack(vertical, controls...)
}

// SetStretchy marks a control in a Stack as stretchy. This cannot be called once the Window containing the Stack has been created.
// It panics if index is out of range.
func (s *Stack) SetStretchy(index int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.created {
		panic("call to Stack.SetStretchy() after Stack has been created")
	}
	if index < 0 || index > len(s.stretchy) {
		panic(fmt.Errorf("index %d out of range in Stack.SetStretchy()", index))
	}
	s.stretchy[index] = true
}

func (s *Stack) make(window *sysData) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for i, c := range s.controls {
		err := c.make(window)
		if err != nil {
			return fmt.Errorf("error adding control %d to Stack: %v", i, err)
		}
	}
	s.created = true
	return nil
}

func (s *Stack) allocate(x int, y int, width int, height int, d *sysSizeData) (allocations []*allocation) {
	var stretchywid, stretchyht int
	var current *allocation		// for neighboring

	if len(s.controls) == 0 { // do nothing if there's nothing to do
		return nil
	}
	// before we do anything, steal the margin so nested Stacks/Grids don't double down
	xmargin := d.xmargin
	ymargin := d.ymargin
	d.xmargin = 0
	d.ymargin = 0
	// 0) inset the available rect by the margins and needed padding
	x += xmargin
	y += ymargin
	width -= xmargin * 2
	height -= ymargin * 2
	if s.orientation == horizontal {
		width -= (len(s.controls) - 1) * d.xpadding
	} else {
		height -= (len(s.controls) - 1) * d.ypadding
	}
	// 1) get height and width of non-stretchy controls; figure out how much space is alloted to stretchy controls
	stretchywid = width
	stretchyht = height
	nStretchy := 0
	for i, c := range s.controls {
		if s.stretchy[i] {
			nStretchy++
			continue
		}
		w, h := c.preferredSize(d)
		if s.orientation == horizontal { // all controls have same height
			s.width[i] = w
			s.height[i] = height
			stretchywid -= w
		} else { // all controls have same width
			s.width[i] = width
			s.height[i] = h
			stretchyht -= h
		}
	}
	// 2) figure out size of stretchy controls
	if nStretchy != 0 {
		if s.orientation == horizontal { // split rest of width
			stretchywid /= nStretchy
		} else { // split rest of height
			stretchyht /= nStretchy
		}
	}
	for i := range s.controls {
		if !s.stretchy[i] {
			continue
		}
		s.width[i] = stretchywid
		s.height[i] = stretchyht
	}
	// 3) now actually place controls
	for i, c := range s.controls {
		as := c.allocate(x, y, s.width[i], s.height[i], d)
		if s.orientation == horizontal {		// no vertical neighbors
			if current != nil {			// connect first left to first right
				current.neighbor = c
			}
			if len(as) != 0 {
				current = as[0]			// next left is first subwidget
			} else {
				current = nil			// spaces don't have allocation data
			}
		}
		allocations = append(allocations, as...)
		if s.orientation == horizontal {
			x += s.width[i] + d.xpadding
		} else {
			y += s.height[i] + d.ypadding
		}
	}
	return allocations
}

// The preferred size of a Stack is the sum of the preferred sizes of non-stretchy controls + (the number of stretchy controls * the largest preferred size among all stretchy controls).
// We don't consider the margins here, but will need to if Window.SizeToFit() is ever made a thing.
func (s *Stack) preferredSize(d *sysSizeData) (width int, height int) {
	max := func(a int, b int) int {
		if a > b {
			return a
		}
		return b
	}

	var nStretchy int
	var maxswid, maxsht int

	if len(s.controls) == 0 { // no controls, so return emptiness
		return 0, 0
	}
	if s.orientation == horizontal {
		width = (len(s.controls) - 1) * d.xpadding
	} else {
		height = (len(s.controls) - 1) * d.ypadding
	}
	for i, c := range s.controls {
		w, h := c.preferredSize(d)
		if s.stretchy[i] {
			nStretchy++
			maxswid = max(maxswid, w)
			maxsht = max(maxsht, h)
		}
		if s.orientation == horizontal { // max vertical size
			if !s.stretchy[i] {
				width += w
			}
			height = max(height, h)
		} else {
			width = max(width, w)
			if !s.stretchy[i] {
				height += h
			}
		}
	}
	if s.orientation == horizontal {
		width += nStretchy * maxswid
	} else {
		height += nStretchy * maxsht
	}
	return
}

func (s *Stack) commitResize(c *allocation, d *sysSizeData) {
	// this is to satisfy Control; nothing to do here
}

func (s *Stack) getAuxResizeInfo(d *sysSizeData) {
	// this is to satisfy Control; nothing to do here
}


// Space returns a null Control intended for padding layouts with blank space.
// It appears to its owner as a Control of 0x0 size.
//
// For a Stack, Space can be used to insert spaces in the beginning or middle of Stacks (Stacks by nature handle spaces at the end themselves). In order for this to work properly, make the Space stretchy.
//
// For a Grid, Space can be used to have an empty cell. A stretchy Grid cell with a Space can be used to anchor the perimeter of a Grid to the respective Window edges without making one of the other controls stretchy instead (leaving empty space in the Window otherwise). Otherwise, you do not need to do anything special for the Space to work (though remember that an entire row or column of Spaces will appear as having height or width zero, respectively, unless one is marked as stretchy).
//
// The value returned from Space() may or may not be unique.
func Space() Control {
	return space
}

// As above, a Stack with no controls draws nothing and reports no errors; its parent will still size it properly if made stretchy.
var space Control = newStack(horizontal)
