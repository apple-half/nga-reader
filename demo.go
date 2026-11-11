package main

import (
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"time"
)

func demoTopics() nga.TopicPage {
	titles := []string{"有没有那种，周末只想慢慢过的时刻？", "桌面收拾干净以后，工作心情真的会变好", "你们通勤的时候都在听什么？", "冰箱里只剩鸡蛋和番茄，可以做什么？", "最近想找一个不用赶进度的单机游戏", "那些买回来不贵，却一直在用的小东西"}
	authors := []string{"山间来信", "薄荷气泡", "慢速列车", "一碗热汤", "存档以后", "口袋清单"}
	counts := []int{86, 42, 128, 57, 93, 64}
	result := nga.TopicPage{Page: 1, TotalPages: 1, BoardName: "大漩涡"}
	for i, title := range titles {
		result.Topics = append(result.Topics, nga.Topic{ID: int64(1001 + i), FID: -7, Title: title, Author: authors[i], AuthorID: int64(101 + i), Replies: counts[i], PostDate: time.Date(2026, 11, 9, 10, 24-i*5, 0, 0, time.Local), LastPost: time.Date(2026, 11, 9, 11, 0-i*4, 0, 0, time.Local)})
	}
	return result
}
func demoThread(id int64) nga.ThreadPage {
	topics := demoTopics().Topics
	selected := topics[0]
	for _, t := range topics {
		if t.ID == id {
			selected = t
		}
	}
	bodies := map[int64]string{
		1001: "这周事情有点多，周五下班以后就决定，周末什么计划都不做。\n\n早上睡到自然醒，去楼下吃了碗面。回家把积了很久的书翻了几页，又在窗边坐了一会儿。\n\n没干什么“大事”，但好像比出去玩一天更放松。\n大家有没有自己的周末充电方式？",
		1002: "周末终于把桌面理了一遍。收掉了很久不用的线，只留下显示器、键盘和台灯。\n\n[b]最有效的改动其实不是买东西，而是少放东西。[/b]\n\n顺便把常用文件也整理好了。现在打开电脑，总算没有那种找不到东西的烦躁感。",
		1003: "每天坐地铁差不多四十分钟，最近想换点不一样的东西听。\n\n以前基本都是歌单循环，后来发现听一些闲聊节目，通勤时间会过得快一点。\n\n大家是听音乐、播客，还是直接戴着耳机放空？",
		1004: "不想点外卖，也不太想出门。翻了一下冰箱，主角只有鸡蛋和番茄。\n\n除了番茄炒蛋，还能做什么简单一点的？家里有面条和米饭，调料也基本都有。\n\n最好是二十分钟以内能吃上的那种。",
		1005: "不太想做日常和赶活动了。想找一个可以随时打开、玩半小时就放下的游戏。\n\n比起很强的剧情推进，更想要一点探索和自己决定节奏的空间。\n\n这次想把进度条忘掉，随便玩玩就好。",
		1006: "整理抽屉的时候发现，最常用的居然都是买的时候没怎么犹豫的小东西。\n\n比如一个顺手的开瓶器、一条够长的充电线，还有书桌旁边的挂钩。\n\n你们有什么买回来以后觉得“早该买了”的东西？"}
	replies := map[int64][2]string{1001: {"我现在最喜欢的安排：买点菜，做一顿不赶时间的饭。吃完慢慢收拾，再泡一杯茶。", "[quote]没干什么“大事”，但好像比出去玩一天更放松。[/quote]\n完全懂这种感觉。有时候休息就是不需要把每一段时间都安排满。"}, 1002: {"显示器下面那一小片空出来的地方，真的会让人舒服很多。", "[quote]最有效的改动其实不是买东西，而是少放东西。[/quote]\n我把桌面上十几个“新建文件夹”整理掉，效果差不多。"}, 1003: {"去公司的时候听音乐，下班的时候什么也不听。每天留一点不用接收信息的时间。", "降噪一开，什么都不放，就是自己的小世界。"}}
	r := replies[id]
	if r[0] == "" {
		r = [2]string{"慢慢来，找到适合自己的节奏就好。", "先收藏了，回头再来看看大家的分享。"}
	}
	posts := []nga.Post{{ID: 0, Floor: 0, AuthorID: selected.AuthorID, Author: selected.Author, Content: bodies[selected.ID], PostDate: selected.PostDate}, {ID: 1, Floor: 1, AuthorID: 201, Author: "在途", Content: r[0], PostDate: selected.PostDate.Add(7 * time.Minute)}, {ID: 2, Floor: 2, AuthorID: 202, Author: "松风过窗", Content: r[1], PostDate: selected.PostDate.Add(11 * time.Minute)}}
	return nga.ThreadPage{Topic: selected, Posts: posts, Page: 1, TotalPages: 1, TotalRows: 3, MainAuthorID: selected.AuthorID}
}
func (a *reader) useDemo() {
	if !a.demo {
		snapshot := a.state
		snapshot.Boards = append([]local.Board(nil), a.state.Boards...)
		snapshot.Favorites = append([]nga.Topic(nil), a.state.Favorites...)
		snapshot.History = append([]local.Reading(nil), a.state.History...)
		a.liveState = &snapshot
	}
	a.topicGate.Begin(a.ctx)
	a.threadGate.Begin(a.ctx)
	a.favoriteGate.Begin(a.ctx)
	a.loadingFavorites = false
	a.demo = true
	a.loadingTopics = false
	a.loadingThread = false
	a.topicErr = nil
	a.threadErr = nil
	a.mode = "board"
	a.filter = ""
	a.state.Favorites = nil
	a.state.History = nil
	a.board = local.Board{ID: -7, Name: "大漩涡"}
	a.topics = demoTopics()
	a.selectTopic(a.topics.Topics[0])
}
