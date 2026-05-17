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
    blockersOpen: false,
    blockerCount: 0,
    theme: 'light',
    boardLoaded: false,

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
    boardCol: 'todo',

    init() {
      this.bootstrapData();
      this.initTheme();
      this.connectSSE();

      // htmx:afterSettle runs after htmx is done manipulating attributes,
      // so our DOM edits won't be overwritten by htmx's attribute merging.
      document.body.addEventListener('htmx:afterSettle', (e) => {
        if (e.detail.target.id === 'board-container') {
          this.boardLoaded = true;
          this.$nextTick(() => this.initSortable());
          this.updateBlockerCount();
          this.syncBlockedColumnVisibility();
        }
        if (e.detail.target.id === 'drawer-container') {
          this.drawerOpen = true;
          const idEl = e.detail.target.querySelector('[data-card-id]');
          if (idEl) {
            this.drawerCardId = parseInt(idEl.dataset.cardId);
          }
        }
        if (e.detail.target.id === 'blocked-cards') {
          this.updateBlockerCount();
        }
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

    postJSON(url, body) {
      return fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
    },

    // === Blockers ===

    toggleBlockers() {
      this.blockersOpen = !this.blockersOpen;
      this.syncBlockedColumnVisibility();
      if (this.blockersOpen) {
        this.refreshBlockers();
      }
    },

    refreshBlockers() {
      htmx.ajax('GET', '/ui/blockers', {
        target: '#blocked-cards',
        swap: 'innerHTML',
      });
    },

    syncBlockedColumnVisibility() {
      const col = document.getElementById('blocked-column');
      if (col) {
        col.classList.toggle('blocked-hidden', !this.blockersOpen);
      }
    },

    updateBlockerCount() {
      const blockedCards = document.querySelectorAll('#blocked-cards .card-tile');
      this.blockerCount = blockedCards.length;
      const countEl = document.getElementById('blocked-count');
      if (countEl) {
        countEl.textContent = this.blockerCount;
      }
      if (this.blockerCount === 0) {
        this.blockersOpen = false;
        this.syncBlockedColumnVisibility();
      }
    },

    // === SortableJS ===

    initSortable() {
      const columns = document.querySelectorAll('.column-cards[data-status]');
      columns.forEach((column) => {
        if (column._sortable) column._sortable.destroy();
        // Blocked column: cards can be pulled OUT (user resolves blocker)
        // but nothing can be dragged IN — agents escalate to blocked via
        // the drawer's status selector, not drag-and-drop.
        const isBlocked = column.id === 'blocked-cards';
        column._sortable = new Sortable(column, {
          group: { name: 'cards', pull: true, put: !isBlocked },
          animation: 200,
          ghostClass: 'sortable-ghost',
          chosenClass: 'sortable-chosen',
          onEnd: (evt) => this.onCardDrop(evt),
        });
      });
    },

    onCardDrop(evt) {
      const cardEl = evt.item;
      const cardId = cardEl.dataset.cardId;
      const newStatus = evt.to.dataset.status;
      const oldStatus = evt.from.dataset.status;

      if (newStatus === oldStatus) return;

      fetch('/ui/cards/' + cardId + '/status', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'status=' + encodeURIComponent(newStatus),
      }).then((resp) => {
        if (!resp.ok) {
          evt.from.appendChild(cardEl);
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
            // If we dragged OUT of blocked, update blocker state
            if (oldStatus === 'blocked') {
              this.blockerCount = Math.max(0, this.blockerCount - 1);
              if (this.blockerCount === 0) {
                this.blockersOpen = false;
                this.syncBlockedColumnVisibility();
              }
            }
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

    handleCardCreated(card) {
      this.loadBoard();
    },

    handleCardUpdated(card) {
      const cardEl = document.querySelector('[data-card-id="' + card.id + '"]');
      if (!cardEl) {
        this.loadBoard();
        return;
      }

      const oldColumn = cardEl.closest('.column-cards');
      const oldStatus = oldColumn ? oldColumn.dataset.status : null;

      // Transitions involving the blocked column can't use FLIP
      // because the blocked column's position changes when it opens.
      if (card.status === 'blocked' || oldStatus === 'blocked') {
        if (card.status === 'blocked') {
          this.blockerCount++;
          this.blockersOpen = true;
        } else {
          this.blockerCount = Math.max(0, this.blockerCount - 1);
          if (this.blockerCount === 0) {
            this.blockersOpen = false;
          }
        }

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
