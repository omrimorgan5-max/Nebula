document.querySelector('#auth-switch').addEventListener('click', (event) => {
  const button = event.target.closest('[data-mode]');
  if (button) setAuthMode(button.dataset.mode);
});

authForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  authError.hidden = true;
  authSubmit.disabled = true;
  const payload = Object.fromEntries(new FormData(authForm));
  try {
    const user = await api(`/api/auth/${authMode}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    showWorkspace(user);
  } catch (error) {
    authError.textContent = error.message;
    authError.hidden = false;
  } finally {
    authSubmit.disabled = false;
  }
});

document.querySelector('#new-note').addEventListener('click', createNote);
document.querySelector('#empty-create').addEventListener('click', createNote);
noteTitle.addEventListener('input', scheduleSave);
document.querySelector('#note-tags').addEventListener('input', scheduleSave);
editorCanvas.addEventListener('input', scheduleSave);
editorCanvas.addEventListener('keyup', rememberSelection);
editorCanvas.addEventListener('mouseup', rememberSelection);
document.querySelector('#bold-button').addEventListener('mousedown', (event) => event.preventDefault());
document.querySelector('#italic-button').addEventListener('mousedown', (event) => event.preventDefault());
document.querySelector('#font-family').addEventListener('mousedown', rememberSelection);
document.querySelector('#bold-button').addEventListener('click', () => { editorCanvas.focus(); restoreSelection(); document.execCommand('bold'); rememberSelection(); scheduleSave(); });
document.querySelector('#italic-button').addEventListener('click', () => { editorCanvas.focus(); restoreSelection(); document.execCommand('italic'); rememberSelection(); scheduleSave(); });
document.querySelector('#font-family').addEventListener('change', (event) => { editorCanvas.focus(); restoreSelection(); document.execCommand('fontName', false, event.target.value); rememberSelection(); scheduleSave(); });
document.querySelector('#delete-note').addEventListener('click', deleteCurrentNote);
document.querySelector('#dashboard-nav').addEventListener('click', () => switchView('dashboard'));
document.querySelector('#notes-nav').addEventListener('click', () => switchView('notes'));
document.querySelector('#posts-nav').addEventListener('click', () => switchView('posts'));
document.querySelector('#files-nav').addEventListener('click', () => switchView('files'));
document.querySelector('#note-search').addEventListener('input', renderNoteList);
document.querySelector('#pin-note').addEventListener('click', togglePinnedNote);
document.querySelector('#stat-notes').addEventListener('click', () => switchView('notes'));
document.querySelector('#stat-posts').addEventListener('click', () => switchView('posts'));
document.querySelector('#stat-files').addEventListener('click', () => switchView('files'));
document.querySelector('#quick-note').addEventListener('click', () => { switchView('notes'); createNote(); });
document.querySelector('#quick-post').addEventListener('click', async () => {
  switchView('posts', false);
  await loadPosts().catch(showError);
  createPost();
});
document.querySelector('#quick-file').addEventListener('click', () => { switchView('files'); setTimeout(() => document.querySelector('#file-input').click(), 0); });
document.querySelector('#all-notes').addEventListener('click', () => switchView('notes'));
document.querySelector('#all-posts').addEventListener('click', () => switchView('posts'));
document.querySelector('#new-post').addEventListener('click', createPost);
document.querySelector('#post-search').addEventListener('input', renderPosts);
document.querySelector('#empty-post-create').addEventListener('click', createPost);
document.querySelector('#save-post').addEventListener('click', savePost);
document.querySelector('#back-posts').addEventListener('click', closePostEditor);
document.querySelector('#delete-post').addEventListener('click', deletePost);
document.querySelector('#save-profile-settings').addEventListener('click', saveProfileSettings);
document.querySelector('#copy-profile-url').addEventListener('click', copyPublicProfileLink);
document.querySelector('#post-status').addEventListener('change', syncPostPublicControl);
document.querySelector('#profile-public-enabled').addEventListener('change', (event) => {
  if (!event.target.checked) document.querySelector('#profile-show-avatar').checked = false;
});
document.querySelectorAll('.post-filter').forEach((button) => button.addEventListener('click', () => {
  postFilter = button.dataset.postFilter;
  document.querySelectorAll('.post-filter').forEach((filter) => {
    const active = filter === button;
    filter.classList.toggle('active', active);
    filter.setAttribute('aria-selected', String(active));
  });
  renderPosts();
}));
document.querySelector('#file-input').addEventListener('change', (event) => uploadFiles(event.target.files));
document.querySelector('#upload-drop').addEventListener('click', () => document.querySelector('#file-input').click());
document.querySelector('#upload-drop').addEventListener('dragover', (event) => { event.preventDefault(); event.currentTarget.classList.add('dragging'); });
document.querySelector('#upload-drop').addEventListener('dragleave', (event) => event.currentTarget.classList.remove('dragging'));
document.querySelector('#upload-drop').addEventListener('drop', (event) => { event.preventDefault(); event.currentTarget.classList.remove('dragging'); uploadFiles(event.dataTransfer.files); });
document.querySelector('#avatar-input').addEventListener('change', (event) => uploadAvatar(event.target.files[0]));
document.querySelector('#remove-avatar').addEventListener('click', removeAvatar);
document.querySelector('#open-sidebar').addEventListener('click', openSidebar);
document.querySelector('#close-sidebar').addEventListener('click', closeSidebar);
document.querySelector('#sidebar-scrim').addEventListener('click', closeSidebar);
document.addEventListener('selectionchange', rememberSelection);
document.addEventListener('keydown', (event) => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's' && !workspace.hidden) {
    event.preventDefault();
    clearTimeout(saveTimer);
    saveCurrentNote();
  }
});

setAuthMode('signin');
api('/api/me').then(showWorkspace).catch(() => showAuth());
