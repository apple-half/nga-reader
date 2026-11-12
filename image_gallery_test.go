package main

import (
	"math"
	"ngareader/internal/nga"
	"testing"
)

func TestOnDemandGalleryDeduplicatesSafeImagesAndStartsAtClicked(t *testing.T) {
	a, tr := continuousReader(t)
	a.thread.Posts = []nga.Post{{Floor: 0, Content: "[img]https://img.nga.cn/attachments/one.jpg[/img]\n[img]https://evil.example/two.jpg[/img]"}, {Floor: 1, Content: "[quote][img]https://img.nga.cn/attachments/two.jpg[/img][/quote]\n[img]https://img.nga.cn/attachments/one.jpg[/img] [s:ac:哭笑]"}}
	a.resetMarkdown()
	a.openPicture("https://img.nga.cn/attachments/two.jpg")
	if len(a.pictureItems) != 2 || a.pictureIndex != 1 || a.pictureAddress != "https://img.nga.cn/attachments/two.jpg" {
		t.Fatalf("wrong clicked gallery: %v index=%d", a.pictureItems, a.pictureIndex)
	}
	a.navigatePicture(-1)
	if a.pictureIndex != 0 || a.pictureAddress != "https://img.nga.cn/attachments/one.jpg" {
		t.Fatal("previous image did not select address")
	}
	a.navigatePicture(-1)
	if a.pictureIndex != 0 {
		t.Fatal("gallery went below first image")
	}
	a.navigatePicture(1)
	a.navigatePicture(1)
	if a.pictureIndex != 1 {
		t.Fatal("gallery exceeded last image")
	}
	if len(tr.pages) != 1 || a.pictureWeb != nil || a.md.LoadImage != nil {
		t.Fatal("gallery prefetched files or opened engine without window")
	}
	before := a.pictureAddress
	a.openPicture("file:///secret.png")
	if a.pictureAddress != before {
		t.Fatal("unsafe destination selected")
	}
}
func TestImageZoomClampsAndResetsOnNavigation(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.pictureItems = []string{"https://img.nga.cn/attachments/one.jpg", "https://img.nga.cn/attachments/two.jpg"}
	a.pictureIndex = 0
	a.setPictureZoom(99)
	if a.pictureZoom != 4 {
		t.Fatal("zoom unbounded")
	}
	a.setPictureZoom(-3)
	if a.pictureZoom != .25 {
		t.Fatal("zoom vanished")
	}
	a.setPictureZoom(math.NaN())
	if a.pictureZoom != 1 {
		t.Fatal("NaN zoom accepted")
	}
	a.setPictureZoom(2)
	a.navigatePicture(1)
	if a.pictureZoom != 1 {
		t.Fatal("next image inherited old zoom")
	}
}
