/* ============================================
   GIMME DOCS — script.js
   ============================================ */

'use strict';

document.addEventListener('DOMContentLoaded', () => {
  if (typeof hljs !== 'undefined') {
    hljs.highlightAll();
  }

  initTabs('.tabs');
  initTabs('.code-tabs');
  initMobileMenu();
  initActiveNavHighlight();
});

// ---- Generic tab initialisation ----
// Works for both the main .tabs and the hero .code-tabs
function initTabs(selector) {
  const tabGroups = document.querySelectorAll(selector);

  tabGroups.forEach((tabGroup) => {
    const tabs = tabGroup.querySelectorAll('[role="tab"]');
    tabs.forEach((tab) => {
      tab.addEventListener('click', () => activateTab(tab, tabs));
      tab.addEventListener('keydown', (e) => handleTabKeydown(e, tabs));
    });
  });
}

function activateTab(tab, allTabs) {
  allTabs.forEach((t) => {
    t.classList.remove('active');
    t.setAttribute('aria-selected', 'false');
    const panel = document.getElementById(t.getAttribute('aria-controls'));
    if (panel) {
      panel.hidden = true;
      panel.classList.remove('active');
    }
  });

  tab.classList.add('active');
  tab.setAttribute('aria-selected', 'true');
  const target = document.getElementById(tab.getAttribute('aria-controls'));
  if (target) {
    target.hidden = false;
    target.classList.add('active');
  }
}

function handleTabKeydown(e, tabs) {
  const index = Array.from(tabs).indexOf(e.target);
  let newIndex = index;

  if (e.key === 'ArrowRight')      newIndex = (index + 1) % tabs.length;
  else if (e.key === 'ArrowLeft')  newIndex = (index - 1 + tabs.length) % tabs.length;
  else if (e.key === 'Home')       newIndex = 0;
  else if (e.key === 'End')        newIndex = tabs.length - 1;
  else return;

  e.preventDefault();
  tabs[newIndex].focus();
  activateTab(tabs[newIndex], tabs);
}

// ---- Mobile menu ----
function initMobileMenu() {
  const toggle = document.querySelector('.menu-toggle');
  const sidebar = document.getElementById('sidebar');

  if (!toggle || !sidebar) return;

  toggle.addEventListener('click', () => {
    const isOpen = sidebar.classList.toggle('open');
    toggle.setAttribute('aria-expanded', String(isOpen));
  });

  // Close when clicking a sidebar link
  sidebar.querySelectorAll('a').forEach((link) => {
    link.addEventListener('click', () => {
      sidebar.classList.remove('open');
      toggle.setAttribute('aria-expanded', 'false');
    });
  });

  // Close when clicking outside
  document.addEventListener('click', (e) => {
    if (
      sidebar.classList.contains('open') &&
      !sidebar.contains(e.target) &&
      !toggle.contains(e.target)
    ) {
      sidebar.classList.remove('open');
      toggle.setAttribute('aria-expanded', 'false');
      toggle.focus();
    }
  });

  // Close with Escape key
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && sidebar.classList.contains('open')) {
      sidebar.classList.remove('open');
      toggle.setAttribute('aria-expanded', 'false');
      toggle.focus();
    }
  });
}

// ---- Active sidebar link on scroll ----
function initActiveNavHighlight() {
  const sections = document.querySelectorAll('section[id]');
  const navLinks = document.querySelectorAll('.sidebar-link');

  if (!sections.length || !navLinks.length) return;

  const headerHeight =
    parseFloat(
      getComputedStyle(document.documentElement).getPropertyValue('--header-height')
    ) || 64;

  const observer = new IntersectionObserver(
    (entries) => {
      // Record current intersection state for each entry
      entries.forEach((entry) => {
        entry.target.dataset.intersecting = entry.isIntersecting ? 'true' : 'false';
      });

      // Highlight the first (topmost) currently visible section
      const active = [...sections].find(
        (s) => s.dataset.intersecting === 'true'
      );

      if (active) {
        navLinks.forEach((link) => {
          link.classList.toggle(
            'active',
            link.getAttribute('href') === `#${active.id}`
          );
        });
      }
    },
    {
      rootMargin: `-${headerHeight + 24}px 0px -60% 0px`,
      threshold: 0,
    }
  );

  sections.forEach((section) => observer.observe(section));
}
