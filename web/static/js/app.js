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

      // After htmx swaps in the board, initialize SortableJS
      document.body.addEventListener('htmx:afterSwap', (e) => {
        if (e.detail.target.id === 'board-container') {
          this.$nextTick(() => this.initSortable());
          this.updateBlockerCount();
        }
        if (e.detail.target.id === 'drawer-container') {
          this.drawerOpen = true;
          const idEl = e.detail.target.querySelector('[data-card-id]');
          if (idEl) {
            this.drawerCardId = parseInt(idEl.dataset.cardId);
          }
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
      this.blockersOpen = false;
    },

    // === Drawer ===

    closeDrawer() {
      this.drawerOpen = false;
      this.drawerCardId = null;
    },

    // === Blockers ===

    toggleBlockers() {
      this.blockersOpen = !this.blockersOpen;
      if (this.blockersOpen) {
        htmx.trigger(document.body, 'blockers-refresh');
      }
    },

    updateBlockerCount() {
      const blockedCards = document.querySelectorAll('#blocked-cards .card-tile');
      this.blockerCount = blockedCards.length;
      const countEl = document.getElementById('blocked-count');
      if (countEl) {
        countEl.textContent = this.blockerCount;
      }
    },

    // === SortableJS ===

    initSortable() {
      const columns = document.querySelectorAll('.column-cards[data-status]');
      columns.forEach((column) => {
        if (column._sortable) column._sortable.destroy();
        column._sortable = new Sortable(column, {
          group: 'cards',
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
            cardEl.outerHTML = html;
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

      if (oldStatus && oldStatus !== card.status) {
        this.flipCard(cardEl, card);
      } else {
        cardEl.classList.add('highlight');
        setTimeout(() => cardEl.classList.remove('highlight'), 1500);
      }

      if (card.status === 'blocked') {
        this.blockerCount++;
        if (!this.blockersOpen) {
          this.blockersOpen = true;
        }
        htmx.trigger(document.body, 'blockers-refresh');
      } else if (oldStatus === 'blocked') {
        this.blockerCount = Math.max(0, this.blockerCount - 1);
        if (this.blockerCount === 0) {
          this.blockersOpen = false;
        }
        htmx.trigger(document.body, 'blockers-refresh');
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
