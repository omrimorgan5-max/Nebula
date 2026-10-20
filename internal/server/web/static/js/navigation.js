function viewFromPath() {
  return Object.keys(pagePaths).find((view) => pagePaths[view] === window.location.pathname) || 'dashboard';
}

function switchView(view, refresh = true, updateHistory = true) {
  document.querySelector('#dashboard-view').hidden = view !== 'dashboard';
  document.querySelector('#notes-view').hidden = view !== 'notes';
  document.querySelector('#posts-view').hidden = view !== 'posts';
  document.querySelector('#files-view').hidden = view !== 'files';
  for (const name of ['dashboard', 'notes', 'posts', 'files']) {
    const button = document.querySelector(`#${name}-nav`);
    button.classList.toggle('selected', view === name);
    if (view === name) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  }
  if (updateHistory && window.location.pathname !== pagePaths[view]) {
    window.history.pushState({ view }, '', pagePaths[view]);
  }
  if (view === 'dashboard') refreshDashboard().catch(showError);
  if (view === 'files') loadFiles().catch(showError);
  if (view === 'posts' && refresh) loadPosts().catch(showError);
  closeSidebar();
}

function openSidebar() { workspace.classList.add('sidebar-open'); }
function closeSidebar() { workspace.classList.remove('sidebar-open'); }

window.addEventListener('popstate', () => switchView(viewFromPath(), true, false));

authTabs.forEach((tab) => tab.addEventListener('click', () => setAuthMode(tab.id === 'signup-tab' ? 'signup' : 'signin')));
