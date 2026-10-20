async function loadNotes() {
  notes = (await api('/api/notes')).map(restorePendingNote);
  document.querySelector('#note-count').textContent = notes.length;
  renderNoteList();
  const active = notes.find((item) => item.id === activeNoteID);
  if (!active && notes.length) selectNote(notes[0].id);
  else if (!active) showEmptyNotes();
}

function pendingNoteKey(id) {
  return `nebula:pending:${currentUser.username}:${id}`;
}

function readPendingNote(id) {
  try {
    return JSON.parse(localStorage.getItem(pendingNoteKey(id)) || 'null');
  } catch {
    return null;
  }
}

function restorePendingNote(note) {
  const pending = readPendingNote(note.id);
  if (!pending || !pending.savedAt || Date.parse(pending.savedAt) <= Date.parse(note.updatedAt)) return note;
  return { ...note, title: pending.title, content: sanitizeLocalNoteContent(pending.content), tags: pending.tags || [], pending: true };
}

function sanitizeLocalNoteContent(html) {
  const source = document.createElement('template');
  source.innerHTML = html;
  const output = document.createElement('div');
  const allowedTags = new Set(['b', 'strong', 'i', 'em', 'p', 'div', 'br', 'font']);
  const allowedFonts = new Set(['Georgia', 'Arial', 'Trebuchet MS', 'Courier New']);
  function copyNodes(sourceNode, targetNode) {
    for (const child of sourceNode.childNodes) {
      if (child.nodeType === Node.TEXT_NODE) {
        targetNode.append(document.createTextNode(child.textContent));
        continue;
      }
      if (child.nodeType !== Node.ELEMENT_NODE) continue;
      const tag = child.tagName.toLowerCase();
      if (['script', 'style', 'iframe', 'object', 'svg', 'math'].includes(tag)) continue;
      if (!allowedTags.has(tag)) {
        copyNodes(child, targetNode);
        continue;
      }
      const safeElement = document.createElement(tag);
      if (tag === 'font' && allowedFonts.has(child.getAttribute('face'))) {
        safeElement.setAttribute('face', child.getAttribute('face'));
      }
      copyNodes(child, safeElement);
      targetNode.append(safeElement);
    }
  }
  copyNodes(source.content, output);
  return output.innerHTML;
}

function persistPendingNote(id, draft) {
  try {
    localStorage.setItem(pendingNoteKey(id), JSON.stringify({ ...draft, savedAt: new Date().toISOString() }));
    return true;
  } catch {
    return false;
  }
}

function clearPendingNote(id) {
  try {
    localStorage.removeItem(pendingNoteKey(id));
  } catch {
    return;
  }
}

async function refreshDashboard() {
  const data = await api('/api/dashboard');
  document.querySelector('#dashboard-note-count').textContent = data.noteCount;
  document.querySelector('#dashboard-post-count').textContent = data.postCount;
  document.querySelector('#dashboard-file-count').textContent = data.fileCount;
  renderRecentItems('#recent-notes', data.recentNotes, 'note');
  renderRecentItems('#recent-posts', data.recentPosts, 'post');
}

function renderRecentItems(selector, items, kind) {
  const list = document.querySelector(selector);
  list.replaceChildren();
  if (!items.length) {
    const empty = document.createElement('p');
    empty.className = 'recent-empty';
    empty.textContent = kind === 'note' ? 'No notes yet. Start with a small thought.' : 'No posts yet. Save a longer story here.';
    list.append(empty);
    return;
  }
  for (const item of items) {
    const button = document.createElement('button');
    button.className = 'recent-item';
    button.type = 'button';
    button.addEventListener('click', () => {
      if (kind === 'note') {
        switchView('notes');
        selectNote(item.id);
      } else {
        switchView('posts');
        openPost(item.id);
      }
    });
    const copy = document.createElement('span');
    copy.className = 'recent-copy';
    const title = document.createElement('strong');
    title.textContent = item.title;
    const preview = document.createElement('span');
    preview.textContent = kind === 'note' ? plainText(item.content).slice(0, 70) || 'A blank page' : item.content.slice(0, 70) || 'A blank page';
    copy.append(title, preview);
    const meta = document.createElement('span');
    meta.className = 'recent-meta';
    meta.textContent = kind === 'post'
      ? (item.isPublic ? 'Public profile' : item.status === 'published' ? 'Ready · private' : 'Draft')
      : formatDate(item.updatedAt);
    button.append(copy, meta);
    list.append(button);
  }
}
