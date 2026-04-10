// Kkullm Web UI — Alpine.js + SortableJS + SSE

function kkullm() {
  return {
    // State
    viewMode: 'project',
    currentProject: null,
    currentAgent: null,
    drawerOpen: false,
    drawerCardId: null,
    blockersOpen: false,
    blockerCount: 0,
    showClosed: false,
    theme: 'light',

    init() {
      this.initTheme();
      this.connectSSE();

      // htmx:afterSettle runs after htmx is done manipulating attributes,
      // so our DOM edits won't be overwritten by htmx's attribute merging.
      document.body.addEventListener('htmx:afterSettle', (e) => {
        if (e.detail.target.id === 'board-container') {
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

      // Read initial project from the board container's hx-get
      const boardContainer = document.getElementById('board-container');
      if (boardContainer) {
        const hxGet = boardContainer.getAttribute('hx-get');
        if (hxGet) {
          const match = hxGet.match(/project=(\d+)/);
          if (match) this.currentProject = match[1];
        }
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

    // === Drawer ===

    closeDrawer() {
      this.drawerOpen = false;
      this.drawerCardId = null;
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
          resp.text().then((msg) => this.showToast(msg));
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
      // Update blocker state then reload the board — afterSwap will
      // call syncBlockedColumnVisibility() with the fresh DOM.
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

    showToast(message) {
      const container = document.getElementById('toast-container');
      const toast = document.createElement('div');
      toast.className = 'toast';
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => toast.remove(), 4000);
    },
  };
}
