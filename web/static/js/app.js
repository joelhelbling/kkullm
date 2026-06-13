// Kkullm Web UI — Alpine.js + SortableJS + SSE

function kkullm() {
  return {
    // === State ===
    viewMode: 'project',
    currentProject: null,
    currentAgent: null,
    projects: [],
    agents: [],
    drawerOpen: false,
    drawerCardId: null,
    theme: 'light',
    boardLoaded: false,
    inArchive: false,
    altHeld: false,

    // Compose modal
    composeOpen: false,
    composeMode: null, // null | 'card' | 'agent' | 'project'
    composeError: '',
    composeBusy: false,

    // Mobile UI state (wired up in later tasks)
    pickerOpen: false,
    pickerPage: 'projects',
    overflowOpen: false,
    quickCaptureOpen: false,
    quickCaptureBusy: false,
    quickCaptureError: '',
    boardCol: 'todo',

    init() {
      this.bootstrapData();
      this.initTheme();
      this.connectSSE();

      document.addEventListener('keydown', (e) => { if (e.key === 'Alt') this.altHeld = true; });
      document.addEventListener('keyup',   (e) => { if (e.key === 'Alt') this.altHeld = false; });
      window.addEventListener('blur',      ()  => { this.altHeld = false; });

      // htmx:afterSettle runs after htmx is done manipulating attributes,
      // so our DOM edits won't be overwritten by htmx's attribute merging.
      document.body.addEventListener('htmx:afterSettle', (e) => {
        if (e.detail.target.id === 'board-container') {
          this.boardLoaded = true;
          this.$nextTick(() => this.initSortable());
          this.$nextTick(() => this.initBoardPager());
        }
        if (e.detail.target.id === 'drawer-container') {
          this.drawerOpen = true;
          const idEl = e.detail.target.querySelector('[data-card-id]');
          if (idEl) {
            this.drawerCardId = parseInt(idEl.dataset.cardId);
          }
        }
      });

      // Surface server-fired HX-Trigger toasts (e.g. blocker query failures).
      // htmx parses `HX-Trigger: {"showToast": {...}}` into a CustomEvent on
      // the triggering element, which bubbles to body.
      document.body.addEventListener('showToast', (e) => {
        const d = e.detail || {};
        if (d.message) this.showToast(d.message, d.variant);
      });

      // htmx fires `htmx:responseError` for any non-2xx response. Without a
      // listener, loadBoard / drawer fetches that 500 leave
      // stale DOM with no user-visible cue. Surface a toast so the user
      // knows something went wrong.
      document.body.addEventListener('htmx:responseError', (e) => {
        const status = e.detail && e.detail.xhr ? e.detail.xhr.status : '';
        this.showToast('Request failed' + (status ? ' (' + status + ')' : ''), 'error');
      });

      this.initPicker();
    },

    initPicker() {
      const saved = localStorage.getItem('kkullm-picker-page');
      if (saved === 'projects' || saved === 'agents') {
        this.pickerPage = saved;
      }
      this.$watch('pickerOpen', (open) => {
        if (open) this.$nextTick(() => this.syncPickerScroll());
      });
      this.$watch('pickerPage', (page) => {
        localStorage.setItem('kkullm-picker-page', page);
      });
      // Track horizontal scroll position to keep pickerPage in sync with swipe.
      this.$nextTick(() => {
        const container = this.$refs.pickerPages;
        if (!container) return;
        const io = new IntersectionObserver((entries) => {
          for (const entry of entries) {
            if (entry.intersectionRatio >= 0.6) {
              const page = entry.target.dataset.page;
              if (page && page !== this.pickerPage) this.pickerPage = page;
            }
          }
        }, { root: container, threshold: [0.6] });
        container.querySelectorAll('.picker-page').forEach(el => io.observe(el));
      });
    },

    setPickerPage(page) {
      this.pickerPage = page;
      this.$nextTick(() => this.syncPickerScroll());
    },

    syncPickerScroll() {
      const container = this.$refs.pickerPages;
      if (!container) return;
      const target = container.querySelector('[data-page="' + this.pickerPage + '"]');
      if (target) target.scrollIntoView({ behavior: 'smooth', inline: 'start', block: 'nearest' });
    },

    selectProject(id) {
      this.viewMode = 'project';
      this.currentProject = String(id);
      this.pickerOpen = false;
      this.loadBoard();
    },

    selectAgent(id) {
      this.viewMode = 'agent';
      this.currentAgent = String(id);
      this.pickerOpen = false;
      this.loadBoard();
    },

    bootstrapData() {
      const el = document.getElementById('boot-data');
      if (!el) return;
      try {
        const data = JSON.parse(el.textContent);
        this.projects = data.projects || [];
        this.agents = data.agents || [];
        if (data.defaultProjectId) {
          this.currentProject = String(data.defaultProjectId);
        }
        if (this.agents.length > 0) {
          this.currentAgent = String(this.agents[0].id);
        }
      } catch (e) {
        console.warn('boot-data parse failed', e);
      }
    },

    currentViewName() {
      if (this.viewMode === 'agent') {
        const a = this.agents.find(x => String(x.id) === String(this.currentAgent));
        return a ? a.name : '(no agent)';
      }
      const p = this.projects.find(x => String(x.id) === String(this.currentProject));
      return p ? p.name : '(no project)';
    },

    // === Keyboard ===

    handleKeydown(e) {
      const tag = (e.target.tagName || '').toLowerCase();
      const inField = tag === 'input' || tag === 'textarea' || tag === 'select' || e.target.isContentEditable;

      if (e.key === 'Escape') {
        if (this.quickCaptureOpen) { this.quickCaptureOpen = false; return; }
        if (this.pickerOpen) { this.pickerOpen = false; return; }
        if (this.overflowOpen) { this.overflowOpen = false; return; }
        if (this.composeOpen) { this.closeCompose(); return; }
        if (this.drawerOpen) { this.closeDrawer(); return; }
        return;
      }

      if (inField) return;

      if (e.key === 'n' || e.key === 'N') {
        e.preventDefault();
        this.openCompose();
      }
    },

    // === Theme ===

    initTheme() {
      const saved = localStorage.getItem('kkullm-theme');
      if (saved) {
        this.theme = saved;
      } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        this.theme = 'dark';
      }
      document.documentElement.setAttribute('data-theme', this.theme);

      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (!localStorage.getItem('kkullm-theme')) {
          this.theme = e.matches ? 'dark' : 'light';
          document.documentElement.setAttribute('data-theme', this.theme);
        }
      });
    },

    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', this.theme);
      localStorage.setItem('kkullm-theme', this.theme);
    },

    // === Navigation ===

    boardPagerStatuses() {
      return ['considering', 'todo', 'in_flight', 'completed', 'tabled'];
    },

    boardColLabel() {
      const map = {
        considering: 'Considering', todo: 'Todo',
        in_flight: 'In Flight', completed: 'Completed', tabled: 'Tabled',
      };
      return map[this.boardCol] || '';
    },

    boardColCount() {
      const col = document.querySelector('.column[data-status="' + this.boardCol + '"]');
      if (!col) return 0;
      return col.querySelectorAll('.card-tile').length;
    },

    boardScopeKey() {
      return this.viewMode === 'agent' ? 'a:' + this.currentAgent : 'p:' + this.currentProject;
    },

    boardPagerStep(delta) {
      const order = this.boardPagerStatuses();
      const idx = Math.max(0, order.indexOf(this.boardCol));
      const next = order[Math.min(order.length - 1, Math.max(0, idx + delta))];
      this.scrollBoardToColumn(next);
    },

    scrollBoardToColumn(status) {
      const board = this.$refs.board;
      if (!board) return;
      const col = board.querySelector('.column[data-status="' + status + '"]');
      if (!col) return;
      // Optimistic: update indicator immediately so chevron taps feel instant.
      // The IntersectionObserver will confirm or correct after the scroll lands.
      this.boardCol = status;
      col.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
    },

    initBoardPager() {
      if (!window.matchMedia('(max-width: 640px)').matches) return;

      const board = this.$refs.board;
      if (!board) return;

      // Determine landing column.
      const key = 'kkullm-board-col:' + this.boardScopeKey();
      const remembered = localStorage.getItem(key);
      let landing;
      if (remembered) {
        landing = remembered;
      } else {
        const inFlight = board.querySelectorAll('[data-status="in_flight"] .card-tile').length;
        if (inFlight > 0) landing = 'in_flight';
        else landing = 'todo';
      }
      this.boardCol = landing;

      // Scroll without smooth on initial landing.
      const col = board.querySelector('.column[data-status="' + landing + '"]');
      if (col) col.scrollIntoView({ behavior: 'instant', inline: 'center', block: 'nearest' });

      // Observe which column is centered.
      if (this._boardIO) this._boardIO.disconnect();
      this._boardIO = new IntersectionObserver((entries) => {
        let best = null;
        for (const entry of entries) {
          if (entry.intersectionRatio >= 0.6 &&
              (!best || entry.intersectionRatio > best.intersectionRatio)) {
            best = entry;
          }
        }
        if (!best) return;
        const status = best.target.dataset.status;
        if (status && status !== this.boardCol) {
          this.boardCol = status;
          localStorage.setItem('kkullm-board-col:' + this.boardScopeKey(), status);
        }
      }, { root: board, threshold: [0.6] });
      board.querySelectorAll('.column').forEach(el => this._boardIO.observe(el));
    },

    loadBoard() {
      const container = document.getElementById('board-container');
      if (!container) return;

      let url;
      if (this.viewMode === 'agent' && this.currentAgent) {
        url = '/ui/board?agent=' + this.currentAgent;
      } else if (this.currentProject) {
        url = '/ui/board?project=' + this.currentProject;
      } else {
        return;
      }

      this.inArchive = false;
      htmx.ajax('GET', url, { target: '#board-container', swap: 'innerHTML' });
    },

    loadArchived() {
      let url;
      if (this.viewMode === 'agent' && this.currentAgent) {
        url = '/ui/archived?agent=' + this.currentAgent;
      } else if (this.currentProject) {
        url = '/ui/archived?project=' + this.currentProject;
      } else {
        return;
      }
      this.inArchive = true;
      htmx.ajax('GET', url, { target: '#board-container', swap: 'innerHTML' });
    },

    // === Drawer ===

    closeDrawer() {
      this.drawerOpen = false;
      this.drawerCardId = null;
    },

    // === Compose Modal ===

    openCompose(mode) {
      this.composeMode = mode || null;
      this.composeError = '';
      this.composeBusy = false;
      this.composeOpen = true;
      // Focus the first input after Alpine renders the form
      this.$nextTick(() => {
        const input = document.querySelector('.compose-modal input:not([type=radio]):not([type=checkbox]), .compose-modal textarea');
        if (input && input.autofocus) input.focus();
      });
    },

    closeCompose() {
      this.composeOpen = false;
      this.composeMode = null;
      this.composeError = '';
      this.composeBusy = false;
    },

    setComposeMode(mode) {
      this.composeMode = mode;
      this.composeError = '';
      this.$nextTick(() => {
        const input = document.querySelector('.compose-modal input[autofocus]');
        if (input) input.focus();
      });
    },

    appendChip(inputId, value) {
      const input = document.getElementById(inputId);
      if (!input) return;
      const current = (input.value || '').trim();
      const parts = current ? current.split(',').map(s => s.trim()).filter(Boolean) : [];
      if (!parts.includes(value)) {
        parts.push(value);
        input.value = parts.join(', ');
      }
      input.focus();
    },

    async submitCompose(evt, type) {
      this.composeError = '';
      this.composeBusy = true;
      const form = evt.target;
      const fd = new FormData(form);

      try {
        if (type === 'project') {
          const body = {
            name: (fd.get('name') || '').toString().trim(),
            description: (fd.get('description') || '').toString().trim(),
          };
          const resp = await this.postJSON('/api/projects', body);
          const project = await resp.json();
          if (!resp.ok) throw new Error(project.error || 'Could not create project.');
          // Add locally so the nav updates immediately
          if (!this.projects.find(p => p.id === project.id)) {
            this.projects.push({ id: project.id, name: project.name });
            this.projects.sort((a, b) => a.name.localeCompare(b.name));
          }
          this.currentProject = String(project.id);
          this.viewMode = 'project';
          this.showToast('Project "' + project.name + '" created.');
          this.resetComposeForm(form);
          this.closeCompose();
          this.loadBoard();
          return;
        }

        if (type === 'agent') {
          const body = {
            name: (fd.get('name') || '').toString().trim(),
            project: (fd.get('project') || '').toString(),
            bio: (fd.get('bio') || '').toString().trim(),
          };
          const resp = await this.postJSON('/api/agents', body);
          const agent = await resp.json();
          if (!resp.ok) throw new Error(agent.error || 'Could not create agent.');
          if (!this.agents.find(a => a.id === agent.id)) {
            this.agents.push({ id: agent.id, name: agent.name, project: agent.project });
            this.agents.sort((a, b) => a.name.localeCompare(b.name));
          }
          this.showToast('Agent "' + agent.name + '" created.');
          this.resetComposeForm(form);
          this.closeCompose();
          return;
        }

        if (type === 'card') {
          const assignees = (fd.get('assignees') || '').toString()
            .split(',').map(s => s.trim()).filter(Boolean);
          const tags = (fd.get('tags') || '').toString()
            .split(',').map(s => s.trim()).filter(Boolean);
          const body = {
            title: (fd.get('title') || '').toString().trim(),
            body: (fd.get('body') || '').toString().trim(),
            status: (fd.get('status') || 'todo').toString(),
            project: (fd.get('project') || '').toString(),
            assignees,
            tags,
          };
          const resp = await this.postJSON('/api/cards', body);
          const card = await resp.json();
          if (!resp.ok) throw new Error(card.error || 'Could not create card.');
          this.showToast('Card #' + card.id + ' "' + card.title + '" created.');
          this.resetComposeForm(form);
          this.closeCompose();
          // The card_created SSE event will trigger handleCardCreated,
          // but to be safe (and instant), reload the board here too.
          this.loadBoard();
          return;
        }
      } catch (err) {
        this.composeError = err.message || 'Something went wrong.';
      } finally {
        this.composeBusy = false;
      }
    },

    resetComposeForm(form) {
      // Alpine's x-show keeps the form in the DOM, so fields keep their values
      // across modal close/reopen. Clear them so each compose starts fresh.
      form.reset();
      // form.reset() puts the project <select> back to its HTML default, which
      // is whichever option had `selected` set at render time — not necessarily
      // the user's current project. Re-seed it.
      const projectSelect = form.querySelector('[name="project"]');
      if (projectSelect && this.currentProject) {
        const cur = this.projects.find(p => String(p.id) === String(this.currentProject));
        if (cur) projectSelect.value = cur.name;
      }
    },

    openQuickCapture() {
      this.quickCaptureError = '';
      this.quickCaptureBusy = false;
      this.quickCaptureOpen = true;
      this.$nextTick(() => {
        const el = this.$refs.quickCaptureTitle;
        if (el) el.focus();
      });
    },

    async submitQuickCapture(evt) {
      this.quickCaptureError = '';
      this.quickCaptureBusy = true;
      const form = evt.target;
      const fd = new FormData(form);
      try {
        const body = {
          title: (fd.get('title') || '').toString().trim(),
          body: '',
          status: 'considering',
          project: (fd.get('project') || '').toString(),
          assignees: [],
          tags: [],
        };
        const resp = await this.postJSON('/api/cards', body);
        const card = await resp.json();
        if (!resp.ok) throw new Error(card.error || 'Could not add card.');
        form.reset();
        this.quickCaptureOpen = false;
        this.showToast('Added to considering');
      } catch (err) {
        this.quickCaptureError = err.message || 'Something went wrong.';
      } finally {
        this.quickCaptureBusy = false;
      }
    },

    postJSON(url, body) {
      return fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
    },

    // === SortableJS ===

    initSortable() {
      const columns = document.querySelectorAll('.column-cards[data-status]');
      columns.forEach((column) => {
        if (column._sortable) column._sortable.destroy();
        column._sortable = new Sortable(column, {
          group: { name: 'cards', pull: true, put: true },
          animation: 200,
          ghostClass: 'sortable-ghost',
          chosenClass: 'sortable-chosen',
          onStart: (evt) => {
            const oe = evt.originalEvent;
            if (oe && typeof oe.altKey === 'boolean') this.altHeld = oe.altKey;
          },
          onEnd: (evt) => this.onCardDrop(evt),
        });
      });
    },

    // Restore a dropped card to its pre-drag position in the original column.
    // SortableJS has already moved the DOM node to evt.to by the time onEnd
    // fires, so appendChild(cardEl) would drop it at the bottom. Re-insert at
    // the original index instead.
    revertDrag(evt) {
      const cardEl = evt.item;
      const siblings = evt.from.children;
      const idx = evt.oldDraggableIndex;
      if (idx == null || idx >= siblings.length) {
        evt.from.appendChild(cardEl);
      } else {
        evt.from.insertBefore(cardEl, siblings[idx]);
      }
    },

    onCardDrop(evt) {
      const cardEl = evt.item;
      const cardId = cardEl.dataset.cardId;
      const newStatus = evt.to.dataset.status;
      const oldStatus = evt.from.dataset.status;

      if (newStatus === oldStatus) return;

      // Unblock-on-edit: dragging a blocked card to a new status prompts the
      // operator. On confirm we clear the flag atomically with the status
      // change (?unblock=1). On cancel we revert the drag and leave it blocked.
      let unblock = false;
      if (cardEl.dataset.blocked === 'true') {
        if (window.confirm('This card is blocked. Unblock it as you move it to ' + newStatus + '?')) {
          unblock = true;
        } else {
          this.revertDrag(evt);
          return;
        }
      }

      // altHeld is tracked via document keydown/keyup + snapshotted at drag
      // start (onStart), because evt.originalEvent.altKey is unreliable at drop
      // time in Chrome/Safari (it works in Firefox). See #62.
      const force = this.altHeld;

      const qs = [];
      if (unblock) qs.push('unblock=1');
      if (force) qs.push('force=1');
      const url = '/ui/cards/' + cardId + '/status' + (qs.length ? '?' + qs.join('&') : '');
      fetch(url, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'status=' + encodeURIComponent(newStatus),
      }).then((resp) => {
        if (!resp.ok) {
          this.revertDrag(evt);
          resp.text().then((msg) => this.showToast(msg, 'error'));
        } else {
          resp.text().then((html) => {
            // Replace the card tile with the server-rendered HTML, then
            // tell htmx to process the new element so its hx-* attributes
            // (notably hx-get for the drawer) become active.
            const template = document.createElement('template');
            template.innerHTML = html.trim();
            const newEl = template.content.firstElementChild;
            if (newEl) {
              cardEl.replaceWith(newEl);
              htmx.process(newEl);
            }
            this.updateColumnCounts();
          });
        }
      });
    },

    updateColumnCounts() {
      document.querySelectorAll('.column').forEach((col) => {
        const cards = col.querySelectorAll('.card-tile');
        const countEl = col.querySelector('.column-count');
        if (countEl) countEl.textContent = cards.length;
      });
    },

    // === SSE ===

    connectSSE() {
      const source = new EventSource('/api/events');

      source.addEventListener('card_created', (e) => {
        const event = JSON.parse(e.data);
        this.handleCardCreated(event.data);
      });

      source.addEventListener('card_updated', (e) => {
        const event = JSON.parse(e.data);
        this.handleCardUpdated(event.data);
      });

      source.addEventListener('card_deleted', (e) => {
        const event = JSON.parse(e.data);
        this.handleCardDeleted(event.data.id);
      });

      source.addEventListener('comment_created', (e) => {
        const event = JSON.parse(e.data);
        this.handleCommentCreated(event.data);
      });

      source.addEventListener('project_renamed', (e) => {
        // Project/agent lists are server-rendered into the layout;
        // a soft refresh of the board is the best we can do without
        // a dedicated loader. Selector labels will catch up on next
        // full page load.
        this.loadBoard();
      });

      source.addEventListener('project_deleted', (e) => {
        const event = JSON.parse(e.data);
        if (event.data && String(this.currentProject) === String(event.data.id)) {
          this.currentProject = '';
        }
        this.loadBoard();
      });

      source.addEventListener('agent_renamed', (e) => {
        this.loadBoard();
      });

      source.addEventListener('agent_deleted', (e) => {
        this.loadBoard();
      });

      source.addEventListener('dataset_reset', (e) => {
        alert('Database was purged. Reloading…');
        setTimeout(() => location.reload(), 1000);
      });

      source.onerror = () => {
        // EventSource auto-reconnects; no action needed
      };
    },

    // isCardInScope returns true when an SSE card payload belongs in the
    // currently-viewed board.
    isCardInScope(card) {
      if (this.viewMode === 'project') {
        // The "All projects" view shows every project's cards.
        if (this.currentProject === 'all') return true;
        const p = this.projects.find((p) => String(p.id) === String(this.currentProject));
        return !!p && card.project === p.name;
      }
      if (this.viewMode === 'agent') {
        const a = this.agents.find((a) => String(a.id) === String(this.currentAgent));
        return !!a && (card.assignees || []).includes(a.name);
      }
      return true;
    },

    handleCardCreated(card) {
      // Don't reload mid-drag — SortableJS state would be clobbered.
      if (window.Sortable && Sortable.active) return;
      // Skip events for cards outside the current scope (other project or
      // unassigned to current agent). Blocked cards always pass the filter.
      if (!this.isCardInScope(card)) return;
      this.loadBoard();
    },

    handleCardUpdated(card) {
      // Skip mid-drag SSE-driven DOM mutations — the in-flight drag will
      // PATCH on drop and reconcile via the response.
      if (window.Sortable && Sortable.active) return;

      const cardEl = document.querySelector('[data-card-id="' + card.id + '"]');
      if (!cardEl) {
        this.loadBoard();
        return;
      }

      const oldColumn = cardEl.closest('.column-cards');
      const oldStatus = oldColumn ? oldColumn.dataset.status : null;

      // Block-state changes alter the tile's badge markup, which a DOM move
      // can't reproduce. Reload the board so the badge appears/clears, then
      // refresh the drawer if it's open on this card.
      const wasBlocked = cardEl.dataset.blocked === 'true';
      if (wasBlocked !== !!card.blocked) {
        this.loadBoard();
        if (this.drawerOpen && this.drawerCardId === card.id) {
          htmx.ajax('GET', '/ui/cards/' + card.id + '/drawer', {
            target: '#drawer-container',
            swap: 'innerHTML',
          });
        }
        return;
      }

      // Regular status change: FLIP animation between visible columns.
      if (oldStatus && oldStatus !== card.status) {
        this.flipCard(cardEl, card);
      } else {
        cardEl.classList.add('highlight');
        setTimeout(() => cardEl.classList.remove('highlight'), 1500);
      }

      if (this.drawerOpen && this.drawerCardId === card.id) {
        htmx.ajax('GET', '/ui/cards/' + card.id + '/drawer', {
          target: '#drawer-container',
          swap: 'innerHTML',
        });
      }
    },

    flipCard(cardEl, card) {
      const first = cardEl.getBoundingClientRect();

      const newColumn = document.querySelector('.column-cards[data-status="' + card.status + '"]');
      if (!newColumn) {
        this.loadBoard();
        return;
      }

      newColumn.prepend(cardEl);
      const last = cardEl.getBoundingClientRect();

      const dx = first.left - last.left;
      const dy = first.top - last.top;
      cardEl.style.transform = 'translate(' + dx + 'px, ' + dy + 'px)';
      cardEl.style.transition = 'none';

      requestAnimationFrame(() => {
        cardEl.style.transition = 'transform 0.4s ease';
        cardEl.style.transform = '';
        cardEl.addEventListener('transitionend', () => {
          cardEl.style.transition = '';
          cardEl.classList.add('highlight');
          setTimeout(() => cardEl.classList.remove('highlight'), 1500);
          this.updateColumnCounts();
        }, { once: true });
      });
    },

    handleCardDeleted(cardId) {
      const cardEl = document.querySelector('[data-card-id="' + cardId + '"]');
      if (cardEl) {
        cardEl.classList.add('fade-out');
        setTimeout(() => {
          cardEl.remove();
          this.updateColumnCounts();
        }, 300);
      }

      if (this.drawerOpen && this.drawerCardId === cardId) {
        this.closeDrawer();
      }
    },

    handleCommentCreated(comment) {
      if (this.drawerOpen && this.drawerCardId === comment.card_id) {
        htmx.ajax('GET', '/ui/cards/' + comment.card_id + '/drawer', {
          target: '#drawer-container',
          swap: 'innerHTML',
        });
      }

      const cardEl = document.querySelector('[data-card-id="' + comment.card_id + '"]');
      if (cardEl) {
        cardEl.classList.add('highlight');
        setTimeout(() => cardEl.classList.remove('highlight'), 1500);
      }
    },

    // === Toast ===

    showToast(message, variant) {
      const container = document.getElementById('toast-container');
      if (!container) return;
      const toast = document.createElement('div');
      toast.className = 'toast' + (variant === 'error' ? ' toast-error' : '');
      const text = document.createElement('span');
      text.textContent = message;
      toast.appendChild(text);
      container.appendChild(toast);
      setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transition = 'opacity 0.25s';
        setTimeout(() => toast.remove(), 260);
      }, 3600);
    },
  };
}
