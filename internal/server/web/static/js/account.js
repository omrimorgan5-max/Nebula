async function api(path, options = {}) {
  const response = await fetch(path, { credentials: 'same-origin', ...options });
  if (response.status === 204) return null;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || 'Something went wrong. Please try again.');
  return data;
}

function setAuthMode(mode) {
  authMode = mode;
  const signup = mode === 'signup';
  authTabs.forEach((tab) => {
    const selected = tab.id === (signup ? 'signup-tab' : 'signin-tab');
    tab.classList.toggle('active', selected);
    tab.setAttribute('aria-selected', String(selected));
  });
  document.querySelector('#form-kicker').textContent = signup ? 'MAKE YOURSELF AT HOME' : 'WELCOME BACK';
  document.querySelector('#auth-title').innerHTML = signup ? 'A space of<br>your very own.' : 'Pick up where<br>you left off.';
  document.querySelector('#form-subtitle').textContent = signup ? 'A private place for everything on your mind.' : 'Your little corner is right where you left it.';
  document.querySelector('#auth-submit span:first-child').textContent = signup ? 'Create your private space' : 'Sign in to your space';
  document.querySelector('#auth-switch').innerHTML = signup
    ? 'Already have a space? <button type="button" data-mode="signin">Sign in</button>'
    : 'New here? <button type="button" data-mode="signup">Create your account</button>';
  document.querySelector('#password').autocomplete = signup ? 'new-password' : 'current-password';
  authError.hidden = true;
  authError.textContent = '';
}

async function showWorkspace(user) {
  if (!user.theme) user = await api('/api/me');
  currentUser = user;
  authScreen.hidden = true;
  workspace.hidden = false;
  document.querySelector('#account-name').textContent = user.username;
  renderAvatar(user.avatarUrl);
  applyTheme(user.theme || 'light');
  document.querySelector('#today-label').textContent = new Intl.DateTimeFormat(undefined, { weekday: 'short', month: 'short', day: 'numeric' }).format(new Date());
  document.querySelector('#dashboard-greeting').textContent = `A little room to think, ${user.username}.`;
  switchView(viewFromPath(), false, false);
  loadNotes().catch(showError);
  loadPosts().catch(showError);
  refreshDashboard().catch(showError);
  if (viewFromPath() === 'files') loadFiles().catch(showError);
}

function showAuth() {
  clearTimeout(saveTimer);
  clearTimeout(retryTimer);
  retryTimer = null;
  workspace.hidden = true;
  authScreen.hidden = false;
  currentUser = null;
  applyTheme('light');
}

function renderAvatar(url) {
  const initial = currentUser.username.slice(0, 1).toUpperCase();
  const buttonImage = document.querySelector('#avatar-image');
  const dialogImage = document.querySelector('#profile-avatar-image');
  const initials = [document.querySelector('#avatar-initial'), document.querySelector('#profile-avatar-initial')];
  initials.forEach((element) => { element.textContent = initial; element.hidden = Boolean(url); });
  for (const image of [buttonImage, dialogImage]) {
    image.hidden = !url;
    image.onerror = () => { image.hidden = true; initials.forEach((element) => { element.hidden = false; }); };
    if (url) image.src = `${url}?v=${Date.now()}`;
  }
  document.querySelector('#remove-avatar').hidden = !url;
  document.querySelector('#avatar-button').setAttribute('aria-label', url ? 'Change profile picture' : 'Add profile picture');
}

function applyTheme(theme) {
  const dark = theme === 'dark';
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  const button = document.querySelector('#theme-toggle');
  const label = dark ? 'Switch to light theme' : 'Switch to dark theme';
  button.setAttribute('aria-label', label);
  button.title = label;
  button.innerHTML = dark
    ? '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"/></svg>'
    : '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.2 15.4A8.5 8.5 0 0 1 8.6 3.8 8.5 8.5 0 1 0 20.2 15.4Z"/></svg>';
}

async function changeTheme() {
  const previous = currentUser.theme || 'light';
  const next = previous === 'dark' ? 'light' : 'dark';
  currentUser.theme = next;
  applyTheme(next);
  try {
    await api('/api/preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ theme: next }),
    });
  } catch (error) {
    currentUser.theme = previous;
    applyTheme(previous);
    showError(error);
  }
}

async function uploadAvatar(file) {
  if (!file) return;
  const formData = new FormData();
  formData.append('avatar', file);
  try {
    const result = await api('/api/avatar', { method: 'POST', body: formData });
    currentUser.avatarUrl = result.avatarUrl;
    renderAvatar(result.avatarUrl);
    showToast('Profile picture updated.');
  } catch (error) {
    showError(error);
  } finally {
    document.querySelector('#avatar-input').value = '';
  }
}

async function removeAvatar() {
  try {
    await api('/api/avatar', { method: 'DELETE' });
    delete currentUser.avatarUrl;
    renderAvatar('');
    showToast('Profile picture removed.');
  } catch (error) {
    showError(error);
  }
}

function showError(error) {
  if (error.message.includes('Sign in') || error.message.includes('session has expired')) {
    showAuth();
    return;
  }
  showToast(error.message || 'Something went wrong.');
}

function showToast(message) {
  toast.textContent = message;
  toast.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.classList.remove('show'), 3000);
}

function setSaveStatus(status, text) {
  saveState.classList.toggle('saving', status === 'saving');
  saveState.classList.toggle('error', status === 'error');
  saveState.querySelector('span:last-child').textContent = text;
}
