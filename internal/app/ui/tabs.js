/*
 * tabs.js
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The tabs.
 *
 * A server holds as many scores as are open — each one a session, see
 * session.go — and every one of them is a tab here. A tab is the editor
 * itself in a frame, given its score's session in the address, so each keeps
 * everything the editor keeps — the zoom, the selection, where it was on the
 * page — for as long as it is open, and switching tabs loses none of it.
 *
 * The editor in a frame asks this page two things. Where a new score or a
 * file should go: into its own tab when that tab is the empty score gscore
 * opened on, with nothing written in it, and into a tab of its own
 * otherwise (see route). And it hands over what it would have told the Mac
 * application directly, its name and its menus, because only the tab being
 * looked at should be telling the application anything.
 *
 * The application talks to this page as it talked to the editor —
 * gscoreCommand, gscoreMenuTree — and it is passed on to the tab in front.
 *
 * And there is one menu bar, over the tabs, rather than one in each: the
 * menus are the score in front's, so this page shows a copy of that tab's own
 * menu bar, taken again whenever a menu is opened, and sends what is chosen
 * from it to that tab. The tabs hide theirs, as they do inside the Mac
 * application, whose menu bar at the top of the screen is the same idea.
 */

"use strict";

(() => {
  // Framed itself: a page embedding gscore wants the editor, not the tabs.
  if (window.parent !== window) {
    location.replace("/?frame=1");
    return;
  }

  const strip = document.getElementById("tabs");
  const bar = document.getElementById("menubar");
  const frames = document.getElementById("frames");

  // Every tab: its session, its frame and its place in the strip.
  const tabs = [];
  let active = null;
  // Tabs being opened, whose session the server already lists: a sync in
  // that moment would give the same score a second tab.
  let opening = 0;

  if (window.gscoreNativeMenu) document.documentElement.classList.add("native-menu");
  // The application has a menu bar of its own; anywhere else, this page
  // draws one.
  const drawsMenus = !window.gscoreNativeMenu;

  // api names a session in a request, the way the editor in a tab does.
  // The first score is the server's default session and is named by
  // naming none.
  const api = (path, token) =>
    !token || token === "default"
      ? path
      : path + (path.includes("?") ? "&" : "?") + "session=" + encodeURIComponent(token);

  const frameAddress = (token, run) => {
    const q = new URLSearchParams({ frame: "1" });
    if (token && token !== "default") q.set("session", token);
    if (run) q.set("run", run);
    return "/?" + q.toString();
  };

  const native = (message) => {
    try {
      window.webkit?.messageHandlers?.gscore?.postMessage(message);
    } catch (_) {
      /* not inside the application */
    }
  };

  const tabOf = (win) => tabs.find((t) => t.frame.contentWindow === win) || null;
  const baseName = (name) => String(name || "").split("/").pop();

  function render() {
    strip.hidden = tabs.length < 2;
    strip.replaceChildren(...tabs.map((t) => t.el));
    for (const t of tabs) {
      const on = t === active;
      t.el.classList.toggle("active", on);
      t.el.setAttribute("aria-selected", on ? "true" : "false");
      t.frame.style.visibility = on ? "visible" : "hidden";
      t.frame.style.zIndex = on ? "1" : "0";
    }
    if (active) {
      document.title = (active.name ? baseName(active.name) + " — " : "") + "Gscore";
    }
  }

  function addTab(token, run, from) {
    const frame = document.createElement("iframe");
    frame.src = frameAddress(token, run);
    // The microphone, for recording an instrument into a soundfont, and the
    // screen, for filling it: the editor asks for both from inside its frame.
    frame.allow = "microphone; fullscreen; autoplay; clipboard-read; clipboard-write";

    const el = document.createElement("div");
    el.className = "tab";
    el.setAttribute("role", "tab");
    const name = document.createElement("span");
    name.className = "name";
    name.textContent = "untitled";
    const close = document.createElement("button");
    close.className = "close";
    close.type = "button";
    close.title = "Close this score";
    close.textContent = "×";
    el.append(name, close);

    const tab = { token, frame, el, label: name, name: "", ready: false, waiting: [], from };
    el.addEventListener("mousedown", (e) => {
      if (e.target === close) return;
      select(tab);
    });
    // The middle button closes a tab, as it does in every program with them.
    el.addEventListener("auxclick", (e) => {
      if (e.button === 1) void closeTab(tab);
    });
    close.addEventListener("click", () => void closeTab(tab));

    tabs.push(tab);
    frames.append(frame);
    return tab;
  }

  function select(tab) {
    if (!tab) return;
    const changed = active !== tab;
    active = tab;
    render();
    try {
      tab.frame.contentWindow?.focus();
    } catch (_) {
      /* not loaded yet */
    }
    if (changed) {
      // The application's title bar and menu bar are about the score in
      // front, so they are told whenever that changes.
      if (tab.name) native("title:" + tab.name);
      if (tab.ready) native("menus");
      copyMenus();
    }
  }

  /* ---------- the menu bar ---------- */

  const frameDocument = (tab) => {
    try {
      return tab?.frame.contentDocument || null;
    } catch (_) {
      return null;
    }
  };

  // copyMenus puts the menus of the tab in front in the bar: its own markup,
  // so every entry is drawn as it is there — the marks beside them, the
  // keys, what is greyed out and what is hidden.
  function copyMenus() {
    if (!drawsMenus) return;
    const doc = active?.ready ? frameDocument(active) : null;
    const theirs = doc?.getElementById("menubar");
    if (!theirs) return;
    closeMenus();
    bar.replaceChildren(...[...theirs.children]
      .filter((el) => el.classList.contains("brand") || el.classList.contains("menu"))
      .map((el) => document.importNode(el, true)));
    bar.hidden = false;
    // As big as the editor's own interface is set to be.
    const scale = doc.documentElement.style.getPropertyValue("--ui-scale");
    if (scale) document.documentElement.style.setProperty("--ui-scale", scale);
  }

  const menus = () => [...bar.querySelectorAll(":scope > .menu")];
  const openMenu = () => bar.querySelector(":scope > .menu.open");

  function closeFlyouts(keep) {
    for (const f of bar.querySelectorAll(".menu-items.flyout")) {
      if (keep && f.contains(keep)) continue;
      f.classList.remove("flyout");
    }
  }

  function closeMenus() {
    closeFlyouts(null);
    for (const m of menus()) m.classList.remove("open");
  }

  function showMenu(menu) {
    closeMenus();
    menu.classList.add("open");
  }

  // showFlyout opens a submenu beside the entry that owns it, held to the
  // window. Its panel is fixed to the window, so it is placed here.
  function showFlyout(sub) {
    const items = sub.querySelector(":scope > .menu-items");
    if (!items) return;
    closeFlyouts(sub);
    const r = sub.getBoundingClientRect();
    items.classList.add("flyout");
    let left = r.right - 2;
    if (left + items.offsetWidth > window.innerWidth - 8) {
      left = Math.max(8, r.left - items.offsetWidth + 2);
    }
    let top = r.top - 4;
    const over = top + items.offsetHeight - (window.innerHeight - 8);
    if (over > 0) top = Math.max(8, top - over);
    items.style.left = left + "px";
    items.style.top = top + "px";
  }

  // A command chosen from the bar goes to the tab in front, which is then
  // given the keys back.
  function choose(name) {
    closeMenus();
    window.gscoreCommand(name);
    try {
      active?.frame.contentWindow?.focus();
    } catch (_) {
      /* gone */
    }
  }

  bar.addEventListener("mousedown", (e) => {
    const title = e.target.closest(".menu-title");
    if (!title) return;
    e.preventDefault();
    const menu = title.closest(".menu");
    if (menu.classList.contains("open")) {
      closeMenus();
      return;
    }
    // Taken again as it opens, so what it offers is what the score in front
    // offers now: the files opened since, Undo greyed or not.
    const at = menus().indexOf(menu);
    copyMenus();
    const fresh = menus()[at];
    if (fresh) showMenu(fresh);
  });
  bar.addEventListener("mouseover", (e) => {
    if (!openMenu()) return;
    const title = e.target.closest(".menu-title");
    if (title) {
      const menu = title.closest(".menu");
      if (!menu.classList.contains("open")) showMenu(menu);
      return;
    }
    const sub = e.target.closest(".submenu");
    if (sub && e.target.closest(".submenu-title")) {
      showFlyout(sub);
      return;
    }
    const item = e.target.closest(".menu-items button");
    if (item) closeFlyouts(item);
  });
  bar.addEventListener("click", (e) => {
    const sub = e.target.closest(".submenu-title");
    if (sub) {
      showFlyout(sub.closest(".submenu"));
      return;
    }
    const item = e.target.closest("button[data-action]");
    if (!item || item.disabled) return;
    choose(item.dataset.action);
  });
  // Anywhere else — the tabs, the score, another window — closes it. By
  // the path the press took rather than where its target is now: opening a
  // menu copies the bar again, and the title pressed is no longer in it.
  document.addEventListener("mousedown", (e) => {
    if (!e.composedPath().includes(bar)) closeMenus();
  });
  window.addEventListener("blur", closeMenus);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeMenus();
  });

  // closeTab lets go of a score, asking first about work that is not saved.
  async function closeTab(tab, { ask = true } = {}) {
    if (tabs.length < 2 || !tabs.includes(tab)) return;
    if (ask) {
      const w = tab.frame.contentWindow;
      if (w?.gscoreTab?.offerToSave) {
        // In front, so the question is asked where it can be seen.
        select(tab);
        if (!(await w.gscoreTab.offerToSave())) return;
      }
    }
    try {
      await fetch("/api/session?close=" + encodeURIComponent(tab.token), { method: "POST" });
    } catch (_) {
      /* the server has gone; the tab goes anyway */
    }
    const i = tabs.indexOf(tab);
    tabs.splice(i, 1);
    tab.frame.remove();
    if (active === tab) {
      const back = tab.from && tabs.includes(tab.from) ? tab.from : tabs[Math.min(i, tabs.length - 1)];
      active = null;
      select(back);
    } else {
      render();
    }
  }

  // openTab makes a score of its own on the server and a tab for it, which
  // runs the command that brought it about.
  async function openTab(run) {
    opening++;
    try {
      const res = await fetch("/api/session", { method: "POST" });
      if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
      const d = await res.json();
      const tab = addTab(d.session, run, active);
      select(tab);
      return tab;
    } finally {
      opening--;
    }
  }

  // sessions is what the server has open, in the order of the tabs.
  async function sessions() {
    try {
      const res = await fetch("/api/sessions");
      if (!res.ok) return [];
      return (await res.json()).sessions || [];
    } catch (_) {
      return [];
    }
  }

  // sync gives a tab to every score the server has open that has none —
  // opened by a script, or by another window on the same server. An empty
  // first score beside others is the one gscore opened on, which nobody
  // wants to see a tab for.
  async function sync() {
    if (opening) return;
    const all = await sessions();
    if (opening) return;
    const others = all.filter((s) => !s.default).length;
    for (const s of all) {
      if (tabs.some((t) => t.token === s.session)) continue;
      if (s.default && s.pristine && (others > 0 || tabs.length > 0)) continue;
      addTab(s.session);
    }
    if (!tabs.length) addTab("default");
    if (!active) select(tabs[0]);
    render();
  }

  // What the editor in a tab asks of this page.
  window.gscoreTabs = {
    get nativeMenu() { return !!window.gscoreNativeMenu; },
    get nativePrint() { return !!window.gscoreNativePrint; },

    // route says where a command that brings a score in should run:
    // "here", in the tab that asked, or "elsewhere", which this has seen to.
    // A file already open is the tab it is open in.
    async route(name, win) {
      const tab = tabOf(win);
      if (!tab) return "here";
      if (name.startsWith("open-path:")) {
        const path = name.slice("open-path:".length);
        const open = (await sessions()).find((s) => s.filename === path);
        if (open) {
          let there = tabs.find((t) => t.token === open.session);
          if (!there) there = addTab(open.session);
          select(there);
          return "elsewhere";
        }
      }
      let pristine = false;
      try {
        const res = await fetch(api("/api/info", tab.token));
        pristine = res.ok && (await res.json()).pristine === true;
      } catch (_) {
        /* asked nothing: a tab of its own is the safe answer */
      }
      if (pristine) return "here";
      await openTab(name);
      return "elsewhere";
    },

    // ready is the editor in a tab saying it can be talked to.
    ready(win) {
      const tab = tabOf(win);
      if (!tab) return;
      tab.ready = true;
      for (const name of tab.waiting.splice(0)) win.gscoreCommand?.(name);
      if (tab === active) {
        native("menus");
        copyMenus();
      }
    },

    // title is what the score in a tab is called, with its star.
    title(win, name) {
      const tab = tabOf(win);
      if (!tab) return;
      tab.name = name;
      tab.label.textContent = baseName(name) || "untitled";
      tab.el.title = name;
      if (tab === active) {
        native("title:" + name);
        render();
      }
    },

    menus(win) {
      if (tabOf(win) !== active) return;
      native("menus");
      if (!openMenu()) copyMenus();
    },

    // native passes on something only the application can do, such as
    // printing, from the tab in front.
    native(win, message) {
      if (tabOf(win) === active) native(message);
    },

    // done is a tab that was opened for a score and got none — the file
    // panel cancelled — going away again, without asking anything.
    done(win) {
      const tab = tabOf(win);
      if (tab) void closeTab(tab, { ask: false });
    },
  };

  // The application, talking to whichever tab is in front. A command sent
  // before the editor in it is up waits for it, rather than being lost: a
  // score double-clicked in the Finder arrives as one.
  window.gscoreCommand = (name) => {
    if (!active) return;
    const w = active.frame.contentWindow;
    if (active.ready && w?.gscoreCommand) w.gscoreCommand(name);
    else active.waiting.push(name);
  };
  window.gscoreMenuTree = () => {
    try {
      return active?.frame.contentWindow?.gscoreMenuTree?.() || [];
    } catch (_) {
      return [];
    }
  };

  // Scores opened on the server some other way get their tabs when the
  // window comes back to the front.
  window.addEventListener("focus", () => void sync());

  void sync();
})();
