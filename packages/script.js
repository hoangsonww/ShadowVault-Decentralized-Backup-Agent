// Progress Bar with RAF for smoothness
const progressBar = document.getElementById('progressBar');
let ticking = false;
let lastKnownScrollPosition = 0;

function updateProgressBar() {
  const windowHeight = window.innerHeight;
  const documentHeight = document.documentElement.scrollHeight - windowHeight;
  const scrolled = lastKnownScrollPosition;
  const progress = Math.min((scrolled / documentHeight) * 100, 100);
  
  if (progressBar) {
    progressBar.style.width = progress + '%';
  }
  
  ticking = false;
}

function requestProgressUpdate() {
  lastKnownScrollPosition = window.pageYOffset;
  
  if (!ticking) {
    window.requestAnimationFrame(updateProgressBar);
    ticking = true;
  }
}

// Update progress bar on scroll with RAF
window.addEventListener('scroll', requestProgressUpdate, { passive: true });
window.addEventListener('resize', requestProgressUpdate, { passive: true });

// Initial update
document.addEventListener('DOMContentLoaded', updateProgressBar);

// Hamburger Menu Toggle
const hamburger = document.getElementById('hamburger');
const navMenu = document.getElementById('navMenu');
const navOverlay = document.getElementById('navOverlay');
const body = document.body;

function toggleMenu(event) {
  if (event) {
    event.stopPropagation();
  }
  
  const isActive = navMenu.classList.contains('active');
  
  if (isActive) {
    closeMenu();
  } else {
    openMenu();
  }
}

function openMenu() {
  hamburger.classList.add('active');
  navMenu.classList.add('active');
  navOverlay.classList.add('active');
  body.style.overflow = 'hidden';
  
  // Add escape key listener
  document.addEventListener('keydown', handleEscKey);
}

function closeMenu() {
  hamburger.classList.remove('active');
  navMenu.classList.remove('active');
  navOverlay.classList.remove('active');
  body.style.overflow = '';
  
  // Remove escape key listener
  document.removeEventListener('keydown', handleEscKey);
}

function handleEscKey(e) {
  if (e.key === 'Escape') {
    closeMenu();
  }
}

if (hamburger) {
  hamburger.addEventListener('click', toggleMenu);
}

if (navOverlay) {
  navOverlay.addEventListener('click', closeMenu);
}

// Close menu when clicking outside
document.addEventListener('click', (e) => {
  if (navMenu && navMenu.classList.contains('active')) {
    if (!navMenu.contains(e.target) && !hamburger.contains(e.target)) {
      closeMenu();
    }
  }
});

// Close menu when window is resized above breakpoint
let resizeTimer;
window.addEventListener('resize', () => {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(() => {
    if (window.innerWidth > 1200) {
      closeMenu();
    }
  }, 250);
});

// Smooth scrolling for navigation links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
  anchor.addEventListener('click', function (e) {
    e.preventDefault();
    const target = document.querySelector(this.getAttribute('href'));
    if (target) {
      // Close mobile menu if open
      closeMenu();
      
      target.scrollIntoView({
        behavior: 'smooth',
        block: 'start'
      });
    }
  });
});

// Scroll to top button
const scrollTopBtn = document.getElementById('scrollTop');

window.addEventListener('scroll', () => {
  if (window.pageYOffset > 300) {
    scrollTopBtn.classList.add('visible');
  } else {
    scrollTopBtn.classList.remove('visible');
  }
});

scrollTopBtn.addEventListener('click', () => {
  window.scrollTo({
    top: 0,
    behavior: 'smooth'
  });
});

// Copy code functionality
function copyCode(button) {
  const codeBlock = button.closest('.code-block');
  const code = codeBlock.querySelector('pre').textContent;
  
  navigator.clipboard.writeText(code).then(() => {
    const originalText = button.textContent;
    button.textContent = '✓ Copied!';
    button.style.background = '#10b981';
    
    setTimeout(() => {
      button.textContent = originalText;
      button.style.background = '';
    }, 2000);
  }).catch(err => {
    console.error('Failed to copy:', err);
    button.textContent = '✗ Failed';
    setTimeout(() => {
      button.textContent = 'Copy';
    }, 2000);
  });
}

// Intersection Observer for fade-in animations
const observerOptions = {
  threshold: 0.1,
  rootMargin: '0px 0px -50px 0px'
};

const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      entry.target.classList.add('fade-in');
      observer.unobserve(entry.target);
    }
  });
}, observerOptions);

// Observe all feature cards and sections
document.addEventListener('DOMContentLoaded', () => {
  const elements = document.querySelectorAll('.feature-card, .tech-item, .command-group, .diagram-container');
  elements.forEach(el => observer.observe(el));
});

// Initialize Mermaid with dark theme
if (typeof mermaid !== 'undefined') {
  mermaid.initialize({
    startOnLoad: true,
    theme: 'dark',
    themeVariables: {
      primaryColor: '#6366f1',
      primaryTextColor: '#f1f5f9',
      primaryBorderColor: '#818cf8',
      lineColor: '#cbd5e1',
      secondaryColor: '#8b5cf6',
      tertiaryColor: '#1e293b',
      background: '#0f172a',
      mainBkg: '#1e293b',
      secondBkg: '#334155',
      border1: '#475569',
      border2: '#64748b',
      note: '#334155',
      noteBkgColor: '#1e293b',
      noteTextColor: '#cbd5e1',
      noteBorderColor: '#475569',
      fontFamily: 'Inter, system-ui, sans-serif',
      fontSize: '14px'
    },
    flowchart: {
      useMaxWidth: true,
      htmlLabels: true,
      curve: 'basis'
    },
    sequence: {
      useMaxWidth: true,
      diagramMarginX: 50,
      diagramMarginY: 10,
      actorMargin: 50,
      width: 150,
      height: 65,
      boxMargin: 10,
      boxTextMargin: 5,
      noteMargin: 10,
      messageMargin: 35,
      mirrorActors: true,
      bottomMarginAdj: 1,
      useMaxWidth: true
    }
  });
}

// Dynamic year for footer
document.addEventListener('DOMContentLoaded', () => {
  const yearElement = document.getElementById('currentYear');
  if (yearElement) {
    yearElement.textContent = new Date().getFullYear();
  }
});

// Tab switching for code examples
function switchTab(tabName, element) {
  const tabContent = document.querySelectorAll('.tab-content');
  const tabButtons = element.parentElement.querySelectorAll('.tab-btn');
  
  tabContent.forEach(content => {
    content.style.display = 'none';
  });
  
  tabButtons.forEach(btn => {
    btn.classList.remove('active');
  });
  
  document.getElementById(tabName).style.display = 'block';
  element.classList.add('active');
}

// Search functionality (basic implementation)
function initSearch() {
  const searchInput = document.getElementById('searchInput');
  if (!searchInput) return;
  
  searchInput.addEventListener('input', (e) => {
    const searchTerm = e.target.value.toLowerCase();
    const sections = document.querySelectorAll('section[id]');
    
    sections.forEach(section => {
      const text = section.textContent.toLowerCase();
      if (text.includes(searchTerm)) {
        section.style.display = '';
      } else {
        section.style.display = searchTerm ? 'none' : '';
      }
    });
  });
}

// Initialize search on load
document.addEventListener('DOMContentLoaded', initSearch);

// Highlight active nav item based on scroll position with smooth transition
function updateActiveNav() {
  const sections = document.querySelectorAll('section[id]');
  const navLinks = document.querySelectorAll('.nav-menu a');
  
  let current = '';
  const scrollPosition = window.pageYOffset;
  const windowHeight = window.innerHeight;
  
  // Find the current section - prioritize sections that are most visible
  let maxVisibleArea = 0;
  
  sections.forEach(section => {
    const sectionTop = section.offsetTop - 100; // Account for header
    const sectionBottom = sectionTop + section.clientHeight;
    const viewportTop = scrollPosition;
    const viewportBottom = scrollPosition + windowHeight;
    
    // Calculate visible area of this section
    const visibleTop = Math.max(sectionTop, viewportTop);
    const visibleBottom = Math.min(sectionBottom, viewportBottom);
    const visibleArea = Math.max(0, visibleBottom - visibleTop);
    
    // If this section has more visible area than previous, mark it as current
    if (visibleArea > maxVisibleArea) {
      maxVisibleArea = visibleArea;
      current = section.getAttribute('id');
    }
  });
  
  // Special case: if at very top, highlight first section
  if (scrollPosition < 300) {
    current = 'overview';
  }
  
  // Update active state for nav links
  navLinks.forEach(link => {
    link.classList.remove('active');
    const href = link.getAttribute('href');
    if (href === `#${current}`) {
      link.classList.add('active');
    }
  });
}

// Throttle function for better performance
function throttle(func, wait) {
  let timeout;
  let lastRan;
  
  return function executedFunction(...args) {
    const context = this;
    
    if (!lastRan) {
      func.apply(context, args);
      lastRan = Date.now();
    } else {
      clearTimeout(timeout);
      timeout = setTimeout(function() {
        if ((Date.now() - lastRan) >= wait) {
          func.apply(context, args);
          lastRan = Date.now();
        }
      }, wait - (Date.now() - lastRan));
    }
  };
}

// Use throttled version for active nav
const throttledUpdateActiveNav = throttle(updateActiveNav, 100);

window.addEventListener('scroll', () => {
  throttledUpdateActiveNav();
}, { passive: true });

document.addEventListener('DOMContentLoaded', () => {
  updateActiveNav();
  updateProgressBar();
});

// Console Easter egg
console.log('%c🔐 ShadowVault', 'font-size: 24px; font-weight: bold; color: #6366f1;');
console.log('%cDecentralized Encrypted Backup Agent', 'font-size: 14px; color: #8b5cf6;');
console.log('%cInterested in contributing? Check out our GitHub!', 'font-size: 12px; color: #94a3b8;');

// Feature card hover effects
document.addEventListener('DOMContentLoaded', () => {
  const cards = document.querySelectorAll('.feature-card');
  
  cards.forEach(card => {
    card.addEventListener('mouseenter', function() {
      this.style.transform = 'translateY(-10px) scale(1.02)';
    });
    
    card.addEventListener('mouseleave', function() {
      this.style.transform = '';
    });
  });
});

// Loading animation for diagrams
document.addEventListener('DOMContentLoaded', () => {
  const diagrams = document.querySelectorAll('.mermaid');
  
  diagrams.forEach(diagram => {
    const loader = document.createElement('div');
    loader.className = 'diagram-loader';
    loader.textContent = 'Loading diagram...';
    loader.style.cssText = 'text-align: center; color: #94a3b8; padding: 2rem;';
    
    if (diagram.childNodes.length === 0) {
      diagram.appendChild(loader);
    }
  });
  
  // Remove loaders after Mermaid renders
  setTimeout(() => {
    document.querySelectorAll('.diagram-loader').forEach(loader => {
      if (loader.previousSibling) {
        loader.remove();
      }
    });
  }, 2000);
});
