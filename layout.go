package main

type readerLayout struct{ Sidebar, Topics float32 }

func layoutFor(width, sidebar, topics float32, hidden bool) readerLayout {
	sidebar = min(280, max(160, sidebar))
	topics = min(450, max(240, topics))
	if hidden {
		sidebar = 0
	}
	if width <= 0 {
		width = 1280
	}
	topics = min(topics, width-sidebar-428)
	if topics < 240 && sidebar > 0 {
		sidebar = max(160, width-668)
		topics = 240
	}
	if width-sidebar-topics-8 < 420 {
		sidebar = 0
		topics = min(450, max(240, width-428))
	}
	return readerLayout{sidebar, topics}
}

func (a *reader) resetLayout() {
	a.state.SidebarWidth, a.state.TopicWidth = 180, 310
	a.state.SidebarHidden = false
	a.focused = false
	a.save()
}
