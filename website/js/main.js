/**
 * Silhouette-DB Website Main JavaScript
 * Handles interactions, animations, and UI effects
 */

// ============================================
// Navbar Scroll Effect
// ============================================
const navbar = document.querySelector('.navbar');

function handleNavbarScroll() {
    if (window.scrollY > 50) {
        navbar.classList.add('scrolled');
    } else {
        navbar.classList.remove('scrolled');
    }
}

window.addEventListener('scroll', handleNavbarScroll);

// ============================================
// Smooth Scroll for Anchor Links
// ============================================
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function(e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            const navHeight = navbar.offsetHeight;
            const targetPosition = target.offsetTop - navHeight;
            
            window.scrollTo({
                top: targetPosition,
                behavior: 'smooth'
            });
            
            // Close mobile menu if open
            const navbarCollapse = document.querySelector('.navbar-collapse');
            if (navbarCollapse.classList.contains('show')) {
                navbarCollapse.classList.remove('show');
            }
        }
    });
});

// ============================================
// Counter Animation for Performance Metrics
// ============================================
class CounterAnimation {
    constructor(element, target, duration = 2000) {
        this.element = element;
        this.target = target;
        this.duration = duration;
        this.startValue = 0;
        this.startTime = null;
        this.hasAnimated = false;
    }
    
    animate(timestamp) {
        if (!this.startTime) this.startTime = timestamp;
        
        const progress = Math.min((timestamp - this.startTime) / this.duration, 1);
        const easeOutQuart = 1 - Math.pow(1 - progress, 4);
        const currentValue = Math.floor(easeOutQuart * this.target);
        
        this.element.textContent = currentValue;
        
        if (progress < 1) {
            requestAnimationFrame(this.animate.bind(this));
        } else {
            this.element.textContent = this.target;
        }
    }
    
    start() {
        if (this.hasAnimated) return;
        this.hasAnimated = true;
        requestAnimationFrame(this.animate.bind(this));
    }
}

// Initialize counters
const counterElements = document.querySelectorAll('.metric-value[data-target]');
const counters = [];

counterElements.forEach(element => {
    const target = parseInt(element.dataset.target);
    counters.push({
        element,
        counter: new CounterAnimation(element, target)
    });
});

// ============================================
// Intersection Observer for Animations
// ============================================
const observerOptions = {
    threshold: 0.2,
    rootMargin: '0px 0px -50px 0px'
};

// Observer for metric cards
const metricObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            // Animate the metric card
            entry.target.classList.add('animate');
            
            // Find and start the counter
            const valueElement = entry.target.querySelector('.metric-value[data-target]');
            if (valueElement) {
                const counterData = counters.find(c => c.element === valueElement);
                if (counterData) {
                    counterData.counter.start();
                }
            }
        }
    });
}, observerOptions);

document.querySelectorAll('.metric-card').forEach(card => {
    metricObserver.observe(card);
});

// Observer for fade-in animations
const fadeObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.classList.add('visible');
            fadeObserver.unobserve(entry.target);
        }
    });
}, { threshold: 0.1 });

// Add fade-in class to elements
document.querySelectorAll('.feature-card, .tech-card, .usecase-card').forEach((element, index) => {
    element.style.opacity = '0';
    element.style.transform = 'translateY(30px)';
    element.style.transition = `opacity 0.6s ease ${index * 0.1}s, transform 0.6s ease ${index * 0.1}s`;
    fadeObserver.observe(element);
});

// Style for visible elements
const style = document.createElement('style');
style.textContent = `
    .feature-card.visible,
    .tech-card.visible,
    .usecase-card.visible {
        opacity: 1 !important;
        transform: translateY(0) !important;
    }
`;
document.head.appendChild(style);

// ============================================
// Code Copy Functionality
// ============================================
function copyCode() {
    const codeBlock = document.querySelector('.code-content code');
    if (!codeBlock) return;
    
    const text = codeBlock.textContent;
    
    navigator.clipboard.writeText(text).then(() => {
        const btn = document.querySelector('.copy-btn');
        const originalIcon = btn.innerHTML;
        btn.innerHTML = '<i class="bi bi-check"></i>';
        btn.style.color = '#00f5d4';
        
        setTimeout(() => {
            btn.innerHTML = originalIcon;
            btn.style.color = '';
        }, 2000);
    }).catch(err => {
        console.error('Failed to copy:', err);
    });
}

// Make copyCode available globally
window.copyCode = copyCode;

// ============================================
// Particle Trail on Mouse Move
// ============================================
class ParticleTrail {
    constructor() {
        this.particles = [];
        this.maxParticles = 20;
        this.isEnabled = window.innerWidth > 768;
        
        if (this.isEnabled) {
            this.init();
        }
        
        window.addEventListener('resize', () => {
            this.isEnabled = window.innerWidth > 768;
        });
    }
    
    init() {
        document.addEventListener('mousemove', (e) => {
            if (!this.isEnabled) return;
            
            // Only create particles on hero section
            const heroSection = document.querySelector('.hero-section');
            if (!heroSection) return;
            
            const rect = heroSection.getBoundingClientRect();
            if (e.clientY < rect.top || e.clientY > rect.bottom) return;
            
            this.createParticle(e.clientX, e.clientY);
        });
        
        this.animate();
    }
    
    createParticle(x, y) {
        if (this.particles.length >= this.maxParticles) {
            const oldParticle = this.particles.shift();
            if (oldParticle.element && oldParticle.element.parentNode) {
                oldParticle.element.parentNode.removeChild(oldParticle.element);
            }
        }
        
        const particle = document.createElement('div');
        particle.className = 'mouse-particle';
        particle.style.cssText = `
            position: fixed;
            pointer-events: none;
            width: 8px;
            height: 8px;
            background: #00f5d4;
            border-radius: 50%;
            left: ${x}px;
            top: ${y}px;
            opacity: 1;
            transition: opacity 0.5s ease, transform 0.5s ease;
            z-index: 9999;
            box-shadow: 0 0 10px rgba(0, 245, 212, 0.5);
        `;
        
        document.body.appendChild(particle);
        
        this.particles.push({
            element: particle,
            life: 1
        });
        
        // Trigger fade out
        setTimeout(() => {
            particle.style.opacity = '0';
            particle.style.transform = 'scale(0)';
        }, 50);
        
        // Remove from DOM
        setTimeout(() => {
            if (particle.parentNode) {
                particle.parentNode.removeChild(particle);
            }
        }, 600);
    }
    
    animate() {
        this.particles = this.particles.filter(p => {
            p.life -= 0.02;
            return p.life > 0;
        });
        
        requestAnimationFrame(() => this.animate());
    }
}

// Initialize particle trail
new ParticleTrail();

// ============================================
// Typing Effect for Hero Badge
// ============================================
class TypeWriter {
    constructor(element, words, wait = 3000) {
        this.element = element;
        this.words = words;
        this.wait = parseInt(wait, 10);
        this.txt = '';
        this.wordIndex = 0;
        this.isDeleting = false;
        this.type();
    }
    
    type() {
        const currentWord = this.words[this.wordIndex % this.words.length];
        
        if (this.isDeleting) {
            this.txt = currentWord.substring(0, this.txt.length - 1);
        } else {
            this.txt = currentWord.substring(0, this.txt.length + 1);
        }
        
        this.element.innerHTML = `<span class="typed-text">${this.txt}</span>`;
        
        let typeSpeed = 100;
        
        if (this.isDeleting) {
            typeSpeed /= 2;
        }
        
        if (!this.isDeleting && this.txt === currentWord) {
            typeSpeed = this.wait;
            this.isDeleting = true;
        } else if (this.isDeleting && this.txt === '') {
            this.isDeleting = false;
            this.wordIndex++;
            typeSpeed = 500;
        }
        
        setTimeout(() => this.type(), typeSpeed);
    }
}

// ============================================
// Active Navigation Highlight
// ============================================
function updateActiveNav() {
    const sections = document.querySelectorAll('section[id]');
    const navLinks = document.querySelectorAll('.nav-link');
    
    let current = '';
    
    sections.forEach(section => {
        const sectionTop = section.offsetTop;
        const sectionHeight = section.offsetHeight;
        
        if (window.scrollY >= sectionTop - 200) {
            current = section.getAttribute('id');
        }
    });
    
    navLinks.forEach(link => {
        link.classList.remove('active');
        if (link.getAttribute('href') === `#${current}`) {
            link.classList.add('active');
        }
    });
}

window.addEventListener('scroll', updateActiveNav);

// ============================================
// Parallax Effect for Hero Section
// ============================================
function parallaxEffect() {
    const heroSection = document.querySelector('.hero-section');
    if (!heroSection) return;
    
    const scrolled = window.scrollY;
    const orbs = document.querySelectorAll('.gradient-orb');
    
    orbs.forEach((orb, index) => {
        const speed = 0.1 + index * 0.05;
        orb.style.transform = `translateY(${scrolled * speed}px)`;
    });
}

window.addEventListener('scroll', () => {
    requestAnimationFrame(parallaxEffect);
});

// ============================================
// Initialize Everything on DOM Load
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    // Add loaded class for animations
    document.body.classList.add('loaded');
    
    // Update active nav on load
    updateActiveNav();
    
    // Add style for active nav link
    const navStyle = document.createElement('style');
    navStyle.textContent = `
        .nav-link.active {
            color: #00f5d4 !important;
        }
        .nav-link.active::after {
            width: 70%;
        }
    `;
    document.head.appendChild(navStyle);
});

// ============================================
// Preloader (optional)
// ============================================
window.addEventListener('load', () => {
    const preloader = document.querySelector('.preloader');
    if (preloader) {
        preloader.classList.add('hidden');
        setTimeout(() => {
            preloader.remove();
        }, 500);
    }
});

// ============================================
// Accessibility: Reduce Motion Support
// ============================================
const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

function handleReducedMotion() {
    if (prefersReducedMotion.matches) {
        document.documentElement.style.setProperty('--transition-fast', '0s');
        document.documentElement.style.setProperty('--transition-normal', '0s');
        document.documentElement.style.setProperty('--transition-slow', '0s');
        
        // Disable particle animations
        document.querySelectorAll('.gradient-orb, .shield-ring, .data-particle').forEach(el => {
            el.style.animation = 'none';
        });
    }
}

prefersReducedMotion.addEventListener('change', handleReducedMotion);
handleReducedMotion();

// ============================================
// Console Easter Egg
// ============================================
console.log(`
%c ██████╗ ██╗██╗     ██╗  ██╗ ██████╗ ██╗   ██╗███████╗████████╗████████╗███████╗
%c██╔════╝ ██║██║     ██║  ██║██╔═══██╗██║   ██║██╔════╝╚══██╔══╝╚══██╔══╝██╔════╝
%c███████╗ ██║██║     ███████║██║   ██║██║   ██║█████╗     ██║      ██║   █████╗  
%c╚════██║ ██║██║     ██╔══██║██║   ██║██║   ██║██╔══╝     ██║      ██║   ██╔══╝  
%c██████╔╝ ██║███████╗██║  ██║╚██████╔╝╚██████╔╝███████╗   ██║      ██║   ███████╗
%c╚═════╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝   ╚═╝      ╚═╝   ╚══════╝
                                                              
%c🔐 Privacy-Preserving Distributed Systems
%c🚀 Check out our GitHub: https://github.com/mundrapranay/silhouette-db
`,
    'color: #00f5d4',
    'color: #00bbf9',
    'color: #9b5de5',
    'color: #f72585',
    'color: #ff6b6b',
    'color: #ffd60a',
    'color: #00f5d4; font-weight: bold',
    'color: #9b5de5'
);

