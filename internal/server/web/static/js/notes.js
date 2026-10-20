function renderNoteList() {
  const list = document.querySelector('#note-list');
  list.replaceChildren();
  if (!notes.length) {
    const empty = document.createElement('div');
    empty.className = 'note-list-empty';
    empty.textContent = 'Your notes will find a home here.';
    list.append(empty);
    return;
  }
  const filter = document.querySelector('#note-search').value.trim().toLowerCase();
  const filteredNotes = notes.filter((item) => !filter || item.title.toLowerCase().includes(filter) || plainText(item.content).toLowerCase().includes(filter) || (item.tags || []).some((tag) => tag.includes(filter)));
  if (!filteredNotes.length) {
    const empty = document.createElement('div');
    empty.className = 'note-list-empty';
    empty.textContent = filter ? 'No matching notes.' : 'Your notes will find a home here.';
    list.append(empty);
    return;
  }
  for (const note of filteredNotes) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `note-list-item${note.id === activeNoteID ? ' active' : ''}`;
    button.addEventListener('click', () => { switchView('notes'); selectNote(note.id); });
    const title = document.createElement('span');
    title.className = 'note-list-title';
    title.textContent = note.title || 'Untitled note';
    const preview = document.createElement('span');
    preview.className = 'note-list-preview';
    preview.textContent = plainText(note.content).slice(0, 55) || 'A blank page';
    button.append(title, preview);
    if (note.tags?.length) {
      const tags = document.createElement('span');
      tags.className = 'content-tag-list';
      tags.textContent = note.tags.map((tag) => `#${tag}`).join(' ');
      button.append(tags);
    }
    list.append(button);
  }
}

function showEmptyNotes() {
  activeNoteID = null;
  document.querySelector('#empty-notes').hidden = false;
  document.querySelector('#editor').hidden = true;
  renderNoteList();
}

function selectNote(id) {
  let selected = notes.find((item) => item.id === id);
  if (!selected) return;
  const pending = readPendingNote(id);
  if (pending && Date.parse(pending.savedAt) > Date.parse(selected.updatedAt)) {
    selected = { ...selected, title: pending.title, content: sanitizeLocalNoteContent(pending.content), tags: pending.tags || [], pending: true };
    notes = notes.map((item) => item.id === id ? selected : item);
  }
  if (activeNoteID && activeNoteID !== id) {
    clearTimeout(saveTimer);
    void saveCurrentNote();
  }
  clearTimeout(saveTimer);
  clearTimeout(retryTimer);
  retryTimer = null;
  activeNoteID = id;
  noteTitle.value = selected.title;
  document.querySelector('#note-tags').value = (selected.tags || []).join(', ');
  editorCanvas.innerHTML = selected.content;
  document.querySelector('#editor').hidden = false;
  document.querySelector('#empty-notes').hidden = true;
  document.querySelector('#edited-label').textContent = `EDITED ${formatDate(selected.updatedAt)}`;
  const pinButton = document.querySelector('#pin-note');
  pinButton.classList.toggle('is-pinned', selected.pinned);
  pinButton.setAttribute('aria-pressed', String(selected.pinned));
  pinButton.querySelector('span').textContent = selected.pinned ? 'Pinned' : 'Pin';
  document.querySelector('#updated-label').textContent = `Last updated ${formatDate(selected.updatedAt).toLowerCase()}.`;
  setSaveStatus(selected.pending ? 'saving' : 'saved', selected.pending ? 'Restored draft · syncing' : 'All changes saved');
  updateWordCount();
  renderNoteList();
  closeSidebar();
  if (selected.pending) saveTimer = setTimeout(saveCurrentNote, 250);
}

function formatDate(value) {
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(value));
}

function plainText(html) {
  const element = document.createElement('div');
  element.innerHTML = html;
  return element.textContent || '';
}

async function createNote() {
  try {
    const note = await api('/api/notes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Untitled note', content: '', tags: [] }),
    });
    notes.unshift(note);
    activeNoteID = note.id;
    renderNoteList();
    refreshDashboard().catch(showError);
    selectNote(note.id);
    noteTitle.focus();
    noteTitle.select();
  } catch (error) {
    showError(error);
  }
}

function scheduleSave() {
  updateWordCount();
  const snapshot = currentNoteSnapshot();
  if (activeNoteID) persistPendingNote(activeNoteID, snapshot);
  setSaveStatus('saving', 'Unsaved changes · saved on device');
  clearTimeout(saveTimer);
  clearTimeout(retryTimer);
  retryTimer = null;
  retryDelay = 1000;
  saveTimer = setTimeout(saveCurrentNote, 700);
}

function currentNoteSnapshot() {
  return {
    title: noteTitle.value.trim() || 'Untitled note',
    content: editorCanvas.innerHTML,
    tags: parseTagsInput(document.querySelector('#note-tags').value),
  };
}

async function saveCurrentNote() {
  if (!activeNoteID) return;
  if (busySaving) {
    saveAgain = true;
    return;
  }
  busySaving = true;
  const id = activeNoteID;
  const snapshot = currentNoteSnapshot();
  const { title, content, tags } = snapshot;
  persistPendingNote(id, snapshot);
  try {
    await api(`/api/notes/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, content, tags }),
    });
    const index = notes.findIndex((item) => item.id === id);
    if (index !== -1) notes[index] = { ...notes[index], ...snapshot, pending: false, updatedAt: new Date().toISOString() };
    const storedDraft = readPendingNote(id);
    if (storedDraft?.title === title && storedDraft?.content === content && JSON.stringify(storedDraft?.tags || []) === JSON.stringify(tags)) {
      clearPendingNote(id);
    }
    retryDelay = 1000;
    clearTimeout(retryTimer);
    retryTimer = null;
    renderNoteList();
    if (activeNoteID === id) {
      setSaveStatus('saved', 'All changes saved');
      document.querySelector('#edited-label').textContent = 'JUST NOW';
      document.querySelector('#updated-label').textContent = 'Saved just now.';
    }
    refreshDashboard().catch(showError);
  } catch (error) {
    const shouldNotify = retryDelay === 1000;
    if (activeNoteID === id) setSaveStatus('error', 'Saved on device · retrying');
    retryTimer = setTimeout(() => {
      if (activeNoteID === id) saveCurrentNote();
    }, retryDelay);
    retryDelay = Math.min(retryDelay * 2, 30000);
    if (activeNoteID === id && shouldNotify) showToast(`Note saved on this device. Retrying server sync: ${error.message}`);
  } finally {
    busySaving = false;
    if (saveAgain) {
      saveAgain = false;
      void saveCurrentNote();
    }
  }
}

async function deleteCurrentNote() {
  if (!activeNoteID || !window.confirm('Delete this note? This cannot be undone.')) return;
  clearTimeout(saveTimer);
  try {
    await api(`/api/notes/${activeNoteID}`, { method: 'DELETE' });
    notes = notes.filter((item) => item.id !== activeNoteID);
    clearPendingNote(activeNoteID);
    activeNoteID = null;
    renderNoteList();
    if (notes.length) selectNote(notes[0].id);
    else showEmptyNotes();
    showToast('Note deleted.');
    refreshDashboard().catch(showError);
  } catch (error) {
    showError(error);
  }
}

async function togglePinnedNote() {
  const item = notes.find((note) => note.id === activeNoteID);
  if (!item) return;
  const pinned = !item.pinned;
  try {
    await api(`/api/notes/${item.id}/pin`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pinned }),
    });
    item.pinned = pinned;
    notes.sort((a, b) => Number(b.pinned) - Number(a.pinned) || new Date(b.updatedAt) - new Date(a.updatedAt));
    renderNoteList();
    document.querySelector('#pin-note').classList.toggle('is-pinned', pinned);
    document.querySelector('#pin-note').setAttribute('aria-pressed', String(pinned));
    document.querySelector('#pin-note span').textContent = pinned ? 'Pinned' : 'Pin';
    refreshDashboard().catch(showError);
  } catch (error) {
    showError(error);
  }
}

function updateWordCount() {
  const words = plainText(editorCanvas.innerHTML).trim().split(/\s+/).filter(Boolean).length;
  document.querySelector('#word-count').textContent = `${words} ${words === 1 ? 'word' : 'words'}`;
}

function rememberSelection() {
  const selection = window.getSelection();
  if (selection && selection.rangeCount && editorCanvas.contains(selection.anchorNode)) savedRange = selection.getRangeAt(0).cloneRange();
}

function restoreSelection() {
  if (!savedRange) return;
  const selection = window.getSelection();
  selection.removeAllRanges();
  selection.addRange(savedRange);
}
