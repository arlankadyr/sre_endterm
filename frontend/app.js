const money = new Intl.NumberFormat('ru-KZ', {
  style: 'currency',
  currency: 'KZT',
  maximumFractionDigits: 0
});

const fallbackImages = [
  'https://images.unsplash.com/photo-1523275335684-37898b6baf30?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1515886657613-9f3515b0c78f?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1560343090-f0409e92791a?auto=format&fit=crop&w=900&q=80'
];

const demoProducts = [
  {
    id: 101,
    name: 'Urban Jacket',
    description: 'Легкая городская куртка с водоотталкивающей тканью и аккуратной посадкой.',
    price: 34900,
    stock: 18,
    category: 'Одежда',
    image_url: fallbackImages[3]
  },
  {
    id: 102,
    name: 'Trail Sneakers',
    description: 'Универсальные кроссовки для прогулок, учебы и повседневных маршрутов.',
    price: 42900,
    stock: 24,
    category: 'Обувь',
    image_url: fallbackImages[1]
  },
  {
    id: 103,
    name: 'Studio Headphones',
    description: 'Беспроводные наушники с чистым звуком, мягкими амбушюрами и долгой батареей.',
    price: 59900,
    stock: 12,
    category: 'Техника',
    image_url: fallbackImages[2]
  },
  {
    id: 104,
    name: 'Daily Backpack',
    description: 'Рюкзак с отделением для ноутбука, плотной спинкой и минималистичным дизайном.',
    price: 27900,
    stock: 30,
    category: 'Аксессуары',
    image_url: fallbackImages[4]
  }
];

const state = {
  token: localStorage.getItem('shopflow_token') || '',
  user: null,
  products: [],
  orders: [],
  adminOrders: [],
  profile: null,
  cart: readCart('guest'),
  route: 'catalog',
  authMode: 'login',
  search: '',
  category: 'all'
};

const els = {};

document.addEventListener('DOMContentLoaded', init);

async function init() {
  cacheElements();
  bindEvents();
  await restoreSession();
  await loadProducts();
  updateAuthUI();
  renderAll();
  refreshIcons();
}

function cacheElements() {
  [
    'openAuthBtn', 'logoutBtn', 'userChip', 'userInitial', 'userEmail', 'cartButton', 'cartCount',
    'statProducts', 'statCart', 'statSession', 'searchInput', 'categoryFilter',
    'productsContainer', 'productsEmpty', 'cartItems', 'cartTotal', 'cartEmpty',
    'clearCartBtn', 'checkoutBtn', 'ordersContainer', 'ordersEmpty', 'refreshOrdersBtn',
    'profileForm', 'profileEmail', 'profileName', 'profilePhone', 'profileAddress',
    'adminProductCount', 'adminOrderCount', 'adminRevenue', 'newProductBtn', 'productForm',
    'productId', 'productName', 'productCategory', 'productPrice', 'productStock',
    'productImage', 'productDescription', 'cancelProductBtn', 'adminProducts', 'adminOrders',
    'authModal', 'closeAuthBtn', 'authTitle', 'authForm', 'authEmail', 'authPassword',
    'authSubmitBtn', 'authHint', 'paymentModal', 'closePaymentBtn', 'paymentTotal',
    'confirmPaymentBtn', 'toast'
  ].forEach((id) => {
    els[id] = document.getElementById(id);
  });
}

function bindEvents() {
  document.querySelectorAll('[data-route]').forEach((node) => {
    node.addEventListener('click', (event) => {
      event.preventDefault();
      showRoute(node.dataset.route);
    });
  });

  els.searchInput.addEventListener('input', (event) => {
    state.search = event.target.value.trim().toLowerCase();
    renderProducts();
  });

  els.categoryFilter.addEventListener('change', (event) => {
    state.category = event.target.value;
    renderProducts();
  });

  els.openAuthBtn.addEventListener('click', openAuth);
  els.closeAuthBtn.addEventListener('click', closeAuth);
  els.authModal.addEventListener('click', closeModalOnBackdrop);
  els.paymentModal.addEventListener('click', closeModalOnBackdrop);
  els.closePaymentBtn.addEventListener('click', closePayment);
  els.logoutBtn.addEventListener('click', () => logout(true));
  els.authForm.addEventListener('submit', handleAuth);

  document.querySelectorAll('[data-auth-mode]').forEach((button) => {
    button.addEventListener('click', () => setAuthMode(button.dataset.authMode));
  });

  document.querySelector('[data-fill-user]').addEventListener('click', () => {
    els.authEmail.value = 'user@test.com';
    els.authPassword.value = '123';
    setAuthMode('login');
  });

  document.querySelector('[data-fill-admin]').addEventListener('click', () => {
    els.authEmail.value = 'admin@shop.com';
    els.authPassword.value = 'admin123';
    setAuthMode('login');
  });

  els.productsContainer.addEventListener('click', handleProductClick);
  els.cartItems.addEventListener('click', handleCartClick);
  els.adminProducts.addEventListener('click', handleProductClick);
  els.adminOrders.addEventListener('click', handleAdminOrderClick);
  els.clearCartBtn.addEventListener('click', clearCart);
  els.checkoutBtn.addEventListener('click', openPayment);
  els.confirmPaymentBtn.addEventListener('click', confirmPayment);
  els.refreshOrdersBtn.addEventListener('click', loadUserOrders);
  els.profileForm.addEventListener('submit', saveProfile);
  els.newProductBtn.addEventListener('click', () => openProductForm());
  els.cancelProductBtn.addEventListener('click', closeProductForm);
  els.productForm.addEventListener('submit', saveProduct);
}

async function restoreSession() {
  if (!state.token) return;

  try {
    const user = await api('/auth/me');
    state.user = normalizeUser(user);
    adoptUserCart();
    await Promise.allSettled([loadProfile(), loadUserOrders(), loadAdminOrders()]);
  } catch (error) {
    logout(false);
  }
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;

  const response = await fetch(path, {
    method: options.method || 'GET',
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined
  });

  const text = await response.text();
  let payload = null;
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch (error) {
      payload = { message: text };
    }
  }

  if (!response.ok) {
    const message = payload?.error || payload?.message || `HTTP ${response.status}`;
    throw new Error(message.trim());
  }

  return payload;
}

async function loadProducts() {
  try {
    const payload = await api('/products');
    const items = Array.isArray(payload) ? payload : payload?.products || [];
    if (!items.length) {
      state.products = demoProducts.map(normalizeProduct);
      renderAll();
      return;
    }
    state.products = items.map(normalizeProduct);
    renderAll();
  } catch (error) {
    showToast('Не удалось загрузить товары: ' + error.message, true);
  }
}

async function loadUserOrders() {
  if (!state.user) return;

  try {
    const payload = await api(`/orders?user_id=${encodeURIComponent(state.user.id)}`);
    const items = Array.isArray(payload) ? payload : payload?.orders || [];
    state.orders = items.map(normalizeOrder);
    renderOrders();
    updateStats();
  } catch (error) {
    showToast('Не удалось загрузить заказы: ' + error.message, true);
  }
}

async function loadAdminOrders() {
  if (!isAdmin()) return;

  try {
    const payload = await api('/orders');
    const items = Array.isArray(payload) ? payload : payload?.orders || [];
    state.adminOrders = items.map(normalizeOrder);
    renderAdminOrders();
    updateStats();
  } catch (error) {
    state.adminOrders = [];
  }
}

async function loadProfile() {
  if (!state.user) return;

  try {
    const profile = await api(`/profiles/${state.user.id}`);
    state.profile = normalizeProfile(profile);
  } catch (error) {
    state.profile = { full_name: '', phone: '', address: '' };
    if (!error.message.toLowerCase().includes('not found')) {
      showToast('Профиль пока пустой, можно заполнить заново.');
    }
  }

  renderProfile();
}

function renderAll() {
  renderCategories();
  renderProducts();
  renderCart();
  renderOrders();
  renderProfile();
  renderAdminProducts();
  renderAdminOrders();
  updateStats();
  refreshIcons();
}

function renderCategories() {
  const current = state.category;
  const categories = [...new Set(state.products.map((product) => product.category).filter(Boolean))].sort();
  els.categoryFilter.innerHTML = [
    '<option value="all">Все категории</option>',
    ...categories.map((category) => `<option value="${escapeHtml(category)}">${escapeHtml(category)}</option>`)
  ].join('');
  els.categoryFilter.value = categories.includes(current) ? current : 'all';
  state.category = els.categoryFilter.value;
}

function renderProducts() {
  const products = filteredProducts();
  els.productsEmpty.classList.toggle('hidden', products.length > 0);
  els.productsContainer.innerHTML = products.map((product) => productCard(product)).join('');
  refreshIcons();
}

function filteredProducts() {
  return state.products.filter((product) => {
    const matchesCategory = state.category === 'all' || product.category === state.category;
    const haystack = `${product.name} ${product.description} ${product.category}`.toLowerCase();
    const matchesSearch = !state.search || haystack.includes(state.search);
    return matchesCategory && matchesSearch;
  });
}

function productCard(product) {
  const disabled = product.stock <= 0 ? 'disabled' : '';
  const stockText = product.stock > 0 ? `${product.stock} шт.` : 'нет в наличии';

  return `
    <article class="product-card">
      <div class="product-media">
        <img src="${escapeAttr(product.image_url)}" alt="${escapeAttr(product.name)}" loading="lazy" onerror="this.src='${fallbackImages[0]}'">
        <span class="stock-pill ${product.stock <= 0 ? 'out' : ''}">${escapeHtml(stockText)}</span>
      </div>
      <div class="product-body">
        <span class="product-category">${escapeHtml(product.category)}</span>
        <h3>${escapeHtml(product.name)}</h3>
        <p>${escapeHtml(product.description)}</p>
        <div class="product-footer">
          <strong class="price">${money.format(product.price)}</strong>
        </div>
        <div class="card-actions">
          <button class="button primary" type="button" data-add-product="${product.id}" ${disabled}>
            <i data-lucide="shopping-cart"></i>
            В корзину
          </button>
          ${isAdmin() ? `
            <button class="icon-button" type="button" data-edit-product="${product.id}" aria-label="Редактировать ${escapeAttr(product.name)}" title="Редактировать">
              <i data-lucide="pencil"></i>
            </button>
            <button class="icon-button" type="button" data-delete-product="${product.id}" aria-label="Удалить ${escapeAttr(product.name)}" title="Удалить">
              <i data-lucide="trash-2"></i>
            </button>
          ` : ''}
        </div>
      </div>
    </article>
  `;
}

function renderCart() {
  const count = cartCount();
  const total = cartTotal();
  els.cartCount.textContent = count;
  els.statCart.textContent = String(count);
  els.cartTotal.textContent = money.format(total);
  els.cartEmpty.classList.toggle('hidden', state.cart.length > 0);

  els.cartItems.innerHTML = state.cart.map((item) => {
    const product = productById(item.productId) || item.snapshot;
    if (!product) return '';
    const image = product.image_url || item.snapshot?.image_url || fallbackImageFor(item.productId);
    const subtotal = product.price * item.quantity;

    return `
      <article class="cart-item">
        <div class="cart-thumb"><img src="${escapeAttr(image)}" alt="${escapeAttr(product.name)}" onerror="this.src='${fallbackImages[0]}'"></div>
        <div>
          <h3>${escapeHtml(product.name)}</h3>
          <p>${money.format(product.price)} x ${item.quantity} = ${money.format(subtotal)}</p>
        </div>
        <div class="row-actions">
          <div class="qty-control" aria-label="Количество">
            <button type="button" data-cart-dec="${item.productId}" aria-label="Уменьшить">-</button>
            <span>${item.quantity}</span>
            <button type="button" data-cart-inc="${item.productId}" aria-label="Увеличить">+</button>
          </div>
          <button class="icon-button" type="button" data-cart-remove="${item.productId}" aria-label="Удалить">
            <i data-lucide="trash-2"></i>
          </button>
        </div>
      </article>
    `;
  }).join('');

  refreshIcons();
}

function renderOrders() {
  els.ordersEmpty.classList.toggle('hidden', state.orders.length > 0);
  els.ordersContainer.innerHTML = state.orders.map((order) => renderOrderCard(order, false)).join('');
  refreshIcons();
}

function renderOrderCard(order, admin) {
  const product = productById(order.product_id);
  const image = product?.image_url || fallbackImageFor(order.product_id);
  const title = product?.name || `Товар #${order.product_id}`;
  const total = order.total_price || (product?.price || 0) * order.quantity;

  return `
    <article class="order-card">
      <div class="order-thumb"><img src="${escapeAttr(image)}" alt="${escapeAttr(title)}" onerror="this.src='${fallbackImages[0]}'"></div>
      <div>
        <span class="status-pill ${escapeAttr(order.status)}">${statusLabel(order.status)}</span>
        <h3>${escapeHtml(title)}</h3>
        <p>Заказ #${order.id} · ${order.quantity} шт. · ${formatDate(order.created_at)}</p>
      </div>
      <div class="row-actions">
        <strong>${money.format(total)}</strong>
        ${admin ? `
          <select data-order-status="${order.id}" aria-label="Статус заказа">
            ${['created', 'paid', 'shipped', 'cancelled'].map((status) => `
              <option value="${status}" ${order.status === status ? 'selected' : ''}>${statusLabel(status)}</option>
            `).join('')}
          </select>
          <button class="button ghost" type="button" data-save-order="${order.id}">
            <i data-lucide="save"></i>
            Статус
          </button>
          <button class="icon-button" type="button" data-delete-order="${order.id}" aria-label="Удалить заказ">
            <i data-lucide="trash-2"></i>
          </button>
        ` : ''}
      </div>
    </article>
  `;
}

function renderProfile() {
  if (!state.user) return;
  els.profileEmail.value = state.user.email || '';
  els.profileName.value = state.profile?.full_name || '';
  els.profilePhone.value = state.profile?.phone || '';
  els.profileAddress.value = state.profile?.address || '';
}

function renderAdminProducts() {
  if (!isAdmin()) {
    els.adminProducts.innerHTML = '';
    return;
  }

  els.adminProducts.innerHTML = state.products.map((product) => `
    <article class="admin-row">
      <div class="admin-thumb"><img src="${escapeAttr(product.image_url)}" alt="${escapeAttr(product.name)}" onerror="this.src='${fallbackImages[0]}'"></div>
      <div>
        <h3>${escapeHtml(product.name)}</h3>
        <p>${escapeHtml(product.category)} · ${money.format(product.price)} · ${product.stock} шт.</p>
      </div>
      <div class="row-actions">
        <button class="button ghost" type="button" data-edit-product="${product.id}">
          <i data-lucide="pencil"></i>
          Редактировать
        </button>
        <button class="button danger" type="button" data-delete-product="${product.id}">
          <i data-lucide="trash-2"></i>
          Удалить
        </button>
      </div>
    </article>
  `).join('');

  refreshIcons();
}

function renderAdminOrders() {
  if (!isAdmin()) {
    els.adminOrders.innerHTML = '';
    return;
  }

  els.adminOrders.innerHTML = state.adminOrders.length
    ? state.adminOrders.map((order) => renderOrderCard(order, true)).join('')
    : '<p class="empty-state">Заказов пока нет.</p>';

  refreshIcons();
}

function updateStats() {
  els.statProducts.textContent = String(state.products.length);
  els.statCart.textContent = String(cartCount());
  els.statSession.textContent = state.user ? (isAdmin() ? 'Admin' : 'User') : 'Гость';
  els.adminProductCount.textContent = String(state.products.length);
  els.adminOrderCount.textContent = String(state.adminOrders.length);
  els.adminRevenue.textContent = money.format(state.adminOrders.reduce((sum, order) => sum + Number(order.total_price || 0), 0));
}

function updateAuthUI() {
  const loggedIn = Boolean(state.user);
  document.querySelectorAll('.auth-only').forEach((node) => node.classList.toggle('hidden', !loggedIn));
  document.querySelectorAll('.admin-only').forEach((node) => node.classList.toggle('hidden', !isAdmin()));
  els.openAuthBtn.classList.toggle('hidden', loggedIn);
  els.logoutBtn.classList.toggle('hidden', !loggedIn);
  els.userChip.classList.toggle('hidden', !loggedIn);

  if (loggedIn) {
    els.userEmail.textContent = state.user.email;
    els.userInitial.textContent = state.user.email.slice(0, 1).toUpperCase();
  }
}

function showRoute(route) {
  if (['orders', 'profile'].includes(route) && !state.user) {
    showToast('Сначала войдите в аккаунт.');
    openAuth();
    route = 'catalog';
  }

  if (route === 'admin' && !isAdmin()) {
    showToast('Нужен вход администратора.', true);
    openAuth();
    route = 'catalog';
  }

  state.route = route;
  document.querySelectorAll('.page-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.page === route);
  });
  document.querySelectorAll('.nav-tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.route === route);
  });

  if (route === 'orders') loadUserOrders();
  if (route === 'profile') loadProfile();
  if (route === 'admin') {
    loadAdminOrders();
    renderAdminProducts();
  }

  refreshIcons();
}

function handleProductClick(event) {
  const add = event.target.closest('[data-add-product]');
  const edit = event.target.closest('[data-edit-product]');
  const del = event.target.closest('[data-delete-product]');

  if (add) {
    addToCart(Number(add.dataset.addProduct));
    return;
  }
  if (edit) {
    const product = productById(Number(edit.dataset.editProduct));
    if (product) openProductForm(product);
    return;
  }
  if (del) {
    deleteProduct(Number(del.dataset.deleteProduct));
  }
}

function handleCartClick(event) {
  const inc = event.target.closest('[data-cart-inc]');
  const dec = event.target.closest('[data-cart-dec]');
  const remove = event.target.closest('[data-cart-remove]');

  if (inc) updateCartQuantity(Number(inc.dataset.cartInc), 1);
  if (dec) updateCartQuantity(Number(dec.dataset.cartDec), -1);
  if (remove) removeFromCart(Number(remove.dataset.cartRemove));
}

async function handleAdminOrderClick(event) {
  const save = event.target.closest('[data-save-order]');
  const del = event.target.closest('[data-delete-order]');

  if (save) {
    const id = Number(save.dataset.saveOrder);
    const select = els.adminOrders.querySelector(`[data-order-status="${id}"]`);
    try {
      await api(`/orders/${id}`, {
        method: 'PUT',
        body: { status: select.value }
      });
      showToast('Статус заказа обновлен.');
      await loadAdminOrders();
      if (state.user) await loadUserOrders();
    } catch (error) {
      showToast('Не удалось обновить заказ: ' + error.message, true);
    }
  }

  if (del) {
    const id = Number(del.dataset.deleteOrder);
    if (!confirm('Удалить заказ?')) return;
    try {
      await api(`/orders/${id}`, { method: 'DELETE' });
      showToast('Заказ удален.');
      await loadAdminOrders();
    } catch (error) {
      showToast('Не удалось удалить заказ: ' + error.message, true);
    }
  }
}

function addToCart(productId) {
  const product = productById(productId);
  if (!product) return;
  if (product.stock <= 0) {
    showToast('Товара нет в наличии.', true);
    return;
  }

  const existing = state.cart.find((item) => item.productId === productId);
  if (existing) {
    existing.quantity += 1;
  } else {
    state.cart.push({
      productId,
      quantity: 1,
      snapshot: {
        name: product.name,
        price: product.price,
        image_url: product.image_url
      }
    });
  }

  saveCart();
  renderCart();
  updateStats();
  showToast(`${product.name} добавлен в корзину.`);
}

function updateCartQuantity(productId, delta) {
  const item = state.cart.find((cartItem) => cartItem.productId === productId);
  if (!item) return;
  item.quantity += delta;
  if (item.quantity <= 0) {
    removeFromCart(productId);
    return;
  }
  saveCart();
  renderCart();
  updateStats();
}

function removeFromCart(productId) {
  state.cart = state.cart.filter((item) => item.productId !== productId);
  saveCart();
  renderCart();
  updateStats();
}

function clearCart() {
  state.cart = [];
  saveCart();
  renderCart();
  updateStats();
}

function openPayment() {
  if (!state.user) {
    showToast('Для оплаты нужно войти или зарегистрироваться.');
    openAuth();
    return;
  }

  if (!state.cart.length) {
    showToast('Корзина пустая.', true);
    return;
  }

  els.paymentTotal.textContent = money.format(cartTotal());
  els.paymentModal.classList.remove('hidden');
  refreshIcons();
}

function closePayment() {
  els.paymentModal.classList.add('hidden');
}

async function confirmPayment() {
  if (!state.user || !state.cart.length) return;

  els.confirmPaymentBtn.disabled = true;
  try {
    for (const item of state.cart) {
      const product = productById(item.productId) || item.snapshot;
      const total = Number(product.price) * item.quantity;
      const order = await api('/orders', {
        method: 'POST',
        body: {
          user_id: Number(state.user.id),
          product_id: Number(item.productId),
          quantity: Number(item.quantity),
          total_price: total,
          status: 'paid'
        }
      });
      const orderId = Number(order?.id || order?.ID);
      if (orderId) {
        await api('/payments', {
          method: 'POST',
          body: { order_id: orderId, amount: total }
        });
      }
    }

    clearCart();
    closePayment();
    showToast('Оплата прошла успешно, заказ создан.');
    await Promise.allSettled([loadUserOrders(), loadAdminOrders()]);
    showRoute('orders');
  } catch (error) {
    showToast('Оплата не завершена: ' + error.message, true);
  } finally {
    els.confirmPaymentBtn.disabled = false;
  }
}

async function handleAuth(event) {
  event.preventDefault();
  const email = els.authEmail.value.trim();
  const password = els.authPassword.value;

  if (!email || !password) return;

  els.authSubmitBtn.disabled = true;
  try {
    if (state.authMode === 'register') {
      await api('/auth/register', {
        method: 'POST',
        body: { email, password }
      });
      showToast('Регистрация прошла успешно.');
    }

    await login(email, password);
    closeAuth();
  } catch (error) {
    showToast(error.message, true);
  } finally {
    els.authSubmitBtn.disabled = false;
  }
}

async function login(email, password) {
  const payload = await api('/auth/login', {
    method: 'POST',
    body: { email, password }
  });

  state.token = payload.token;
  localStorage.setItem('shopflow_token', state.token);
  const me = await api('/auth/me');
  state.user = normalizeUser(me);
  adoptUserCart();
  await Promise.allSettled([loadProfile(), loadUserOrders(), loadAdminOrders()]);
  updateAuthUI();
  renderAll();
  showToast(`Добро пожаловать, ${state.user.email}.`);
}

function logout(withToast) {
  state.token = '';
  state.user = null;
  state.profile = null;
  state.orders = [];
  state.adminOrders = [];
  state.cart = readCart('guest');
  localStorage.removeItem('shopflow_token');
  updateAuthUI();
  showRoute('catalog');
  renderAll();
  if (withToast) showToast('Вы вышли из аккаунта.');
}

function openAuth() {
  els.authModal.classList.remove('hidden');
  els.authEmail.focus();
  refreshIcons();
}

function closeAuth() {
  els.authModal.classList.add('hidden');
}

function setAuthMode(mode) {
  state.authMode = mode;
  document.querySelectorAll('[data-auth-mode]').forEach((button) => {
    button.classList.toggle('active', button.dataset.authMode === mode);
  });
  els.authTitle.textContent = mode === 'login' ? 'Вход' : 'Регистрация';
  els.authSubmitBtn.innerHTML = mode === 'login'
    ? '<i data-lucide="log-in"></i>Войти'
    : '<i data-lucide="user-plus"></i>Зарегистрироваться';
  els.authHint.textContent = mode === 'login'
    ? 'Админ по умолчанию: admin@shop.com / admin123'
    : 'Новый аккаунт создается как обычный пользователь.';
  refreshIcons();
}

function closeModalOnBackdrop(event) {
  if (event.target === els.authModal) closeAuth();
  if (event.target === els.paymentModal) closePayment();
}

async function saveProfile(event) {
  event.preventDefault();
  if (!state.user) return;

  const body = {
    full_name: els.profileName.value.trim(),
    phone: els.profilePhone.value.trim(),
    address: els.profileAddress.value.trim()
  };

  try {
    const profile = await api(`/profiles/${state.user.id}`, {
      method: 'PUT',
      body
    });
    state.profile = normalizeProfile(profile);
    renderProfile();
    showToast('Профиль сохранен.');
  } catch (error) {
    showToast('Не удалось сохранить профиль: ' + error.message, true);
  }
}

function openProductForm(product = null) {
  if (!isAdmin()) return;
  els.productForm.classList.remove('hidden');
  els.productId.value = product?.id || '';
  els.productName.value = product?.name || '';
  els.productCategory.value = product?.category || '';
  els.productPrice.value = product?.price || '';
  els.productStock.value = product?.stock ?? '';
  els.productImage.value = product?.image_url || '';
  els.productDescription.value = product?.description || '';
  els.productName.focus();
  showRoute('admin');
}

function closeProductForm() {
  els.productForm.reset();
  els.productId.value = '';
  els.productForm.classList.add('hidden');
}

async function saveProduct(event) {
  event.preventDefault();
  if (!isAdmin()) return;

  const productId = Number(els.productId.value);
  const body = {
    name: els.productName.value.trim(),
    category: els.productCategory.value.trim(),
    description: els.productDescription.value.trim(),
    price: Number(els.productPrice.value),
    stock: Number(els.productStock.value),
    image_url: els.productImage.value.trim()
  };

  try {
    if (productId) {
      await api(`/products/${productId}`, { method: 'PUT', body });
      showToast('Товар обновлен.');
    } else {
      await api('/products', { method: 'POST', body });
      showToast('Товар добавлен.');
    }
    closeProductForm();
    await loadProducts();
  } catch (error) {
    showToast('Не удалось сохранить товар: ' + error.message, true);
  }
}

async function deleteProduct(productId) {
  if (!isAdmin()) return;
  const product = productById(productId);
  if (!confirm(`Удалить товар "${product?.name || productId}"?`)) return;

  try {
    await api(`/products/${productId}`, { method: 'DELETE' });
    state.cart = state.cart.filter((item) => item.productId !== productId);
    saveCart();
    showToast('Товар удален.');
    await loadProducts();
  } catch (error) {
    showToast('Не удалось удалить товар: ' + error.message, true);
  }
}

function readCart(owner) {
  try {
    return JSON.parse(localStorage.getItem(`shopflow_cart_${owner}`) || '[]');
  } catch (error) {
    return [];
  }
}

function saveCart() {
  const owner = state.user?.id || 'guest';
  localStorage.setItem(`shopflow_cart_${owner}`, JSON.stringify(state.cart));
}

function adoptUserCart() {
  if (!state.user) return;
  const userCart = readCart(state.user.id);
  const guestCart = readCart('guest');
  state.cart = userCart.length ? userCart : guestCart;
  saveCart();
}

function cartCount() {
  return state.cart.reduce((sum, item) => sum + item.quantity, 0);
}

function cartTotal() {
  return state.cart.reduce((sum, item) => {
    const product = productById(item.productId) || item.snapshot;
    return sum + Number(product?.price || 0) * item.quantity;
  }, 0);
}

function productById(id) {
  return state.products.find((product) => Number(product.id) === Number(id));
}

function normalizeUser(user) {
  return {
    id: Number(user.id || user.ID),
    email: user.email || user.Email || '',
    is_admin: Boolean(user.is_admin ?? user.IsAdmin)
  };
}

function normalizeProduct(product, index = 0) {
  const id = Number(product.id || product.ID);
  const name = product.name || product.Name || 'Без названия';
  const category = product.category || product.Category || guessCategory(name);
  const image = product.image_url || product.ImageURL || fallbackImageFor(id || index || name);

  return {
    id,
    name,
    description: product.description || product.Description || '',
    price: Number(product.price ?? product.Price ?? 0),
    stock: Number(product.stock ?? product.Stock ?? 0),
    category,
    image_url: image
  };
}

function normalizeOrder(order) {
  return {
    id: Number(order.id || order.ID),
    user_id: Number(order.user_id ?? order.UserID),
    product_id: Number(order.product_id ?? order.ProductID),
    quantity: Number(order.quantity ?? order.Quantity ?? 1),
    total_price: Number(order.total_price ?? order.TotalPrice ?? 0),
    status: String(order.status || order.Status || 'created').toLowerCase(),
    created_at: order.created_at || order.CreatedAt || ''
  };
}

function normalizeProfile(profile) {
  return {
    full_name: profile.full_name || profile.FullName || '',
    phone: profile.phone || profile.Phone || '',
    address: profile.address || profile.Address || ''
  };
}

function guessCategory(name) {
  const value = String(name).toLowerCase();
  if (value.includes('shoe') || value.includes('sneaker') || value.includes('крос')) return 'Обувь';
  if (value.includes('bag') || value.includes('backpack') || value.includes('сум')) return 'Аксессуары';
  if (value.includes('watch') || value.includes('phone') || value.includes('head')) return 'Техника';
  return 'ShopFlow';
}

function fallbackImageFor(seed) {
  const text = String(seed || 'shopflow');
  const hash = [...text].reduce((sum, char) => sum + char.charCodeAt(0), 0);
  return fallbackImages[hash % fallbackImages.length];
}

function statusLabel(status) {
  const labels = {
    created: 'Создан',
    paid: 'Оплачен',
    shipped: 'Отправлен',
    cancelled: 'Отменен'
  };
  return labels[String(status).toLowerCase()] || status;
}

function formatDate(value) {
  if (!value) return 'сейчас';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'сейчас';
  return new Intl.DateTimeFormat('ru-KZ', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date);
}

function isAdmin() {
  return Boolean(state.user?.is_admin);
}

function showToast(message, isError = false) {
  els.toast.textContent = message;
  els.toast.classList.toggle('error', isError);
  els.toast.classList.remove('hidden');
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => {
    els.toast.classList.add('hidden');
  }, 3200);
}

function refreshIcons() {
  if (window.lucide) {
    window.lucide.createIcons({
      attrs: {
        'stroke-width': 2,
        'aria-hidden': 'true'
      }
    });
  }
}

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function escapeAttr(value) {
  return escapeHtml(value).replaceAll('`', '&#096;');
}
