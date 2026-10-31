// The sApp UI kit for React Native in an Expo app (UI-KIT-SPEC.md). The kit draws the screens; everything that
// belongs to the device stays here in the app: secure storage, the in-app browser, the position, the camera, the
// document picker, the card sheet and navigation. Run with `npx expo start` after `npm install` in this folder.
import { useMemo, useRef, useState } from 'react';
import { Alert, Linking, Pressable, RefreshControl, SafeAreaView, ScrollView, Share, Text, View } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import * as WebBrowser from 'expo-web-browser';
import * as Location from 'expo-location';
import * as ImagePicker from 'expo-image-picker';
import * as DocumentPicker from 'expo-document-picker';
import * as FileSystem from 'expo-file-system';
import { SuperrouteProvider, createPartnerClient, createSecureTokenStore, themeFromBranding, useSession } from '@superrouteca/sapp-react-native';
import {
  AccountPrivacy, Cart, Checkout, DriverDuty, DriverEarnings, DriverRoutes, GroupBuy, GroupBuyLeading, GroupBuyLeadingGroup, GroupBuyOrder, GroupBuyOrders,
  NotificationInbox, NotificationPreferences, OrderDetail, OrderList, ProductDetail, ReturnForm, ReturnList, RouteStops, ScanLookup,
  ShippingOrderForm, SignIn, Stop, StoreDirectory, StorePage, Tracking, WalletSummary, configure,
} from '@superrouteca/sapp-react-native/ui';

const REDIRECT_URI = 'sappexample://signed-in';

const tokenStore = createSecureTokenStore({
  getItem: (k) => SecureStore.getItemAsync(k),
  setItem: (k, v) => SecureStore.setItemAsync(k, v),
  removeItem: (k) => SecureStore.deleteItemAsync(k),
});

// Your app code from the Open Platform console, the service provider's code and the app key.
const client = createPartnerClient({ appCode: 'p_your_app', clientCode: 'your_client', appKey: 'pk_your_key', tokenStore, locale: 'en' });

// Defaults for every component: the provider's branding as the theme, the language.
configure({ theme: themeFromBranding({ primary_color: '#1A3D7C' }), locale: 'en' });

/** The sign-in page in an in-app browser; answers the redirect URL, or null when the person closed it. */
async function openBrowser(url, redirectUri) {
  const result = await WebBrowser.openAuthSessionAsync(url, redirectUri);
  return result.type === 'success' ? result.url : null;
}

async function getPosition() {
  const { status } = await Location.requestForegroundPermissionsAsync();
  if (status !== 'granted') return null;
  const { coords } = await Location.getCurrentPositionAsync({});
  return { latitude: coords.latitude, longitude: coords.longitude };
}

/** A photo from the camera: a file on the device, sent as it is. */
async function pickPhoto() {
  const result = await ImagePicker.launchCameraAsync({ mediaTypes: ['images'], quality: 0.6 });
  if (result.canceled) return null;
  const asset = result.assets[0];
  return { uri: asset.uri, name: asset.fileName ?? 'photo.jpg', type: asset.mimeType ?? 'image/jpeg', size: asset.fileSize };
}

/** A label or barcode file for a return, read as base64. */
async function pickFile() {
  const result = await DocumentPicker.getDocumentAsync({ copyToCacheDirectory: true });
  if (result.canceled) return null;
  const asset = result.assets[0];
  const base64 = await FileSystem.readAsStringAsync(asset.uri, { encoding: FileSystem.EncodingType.Base64 });
  return { name: asset.name, base64, size: asset.size };
}

const SCREENS = ['Inbox', 'Wallet', 'Orders', 'Tracking', 'Returns', 'Ship', 'Stores', 'Group buys', 'Account', 'Driver'];

function Tabs({ value, onChange }) {
  return (
    <ScrollView horizontal style={{ flexGrow: 0 }} contentContainerStyle={{ gap: 8, padding: 8 }}>
      {SCREENS.map((name) => (
        <Pressable key={name} onPress={() => onChange(name)} accessibilityRole="button" accessibilityState={{ selected: value === name }}
          style={{ minHeight: 44, paddingHorizontal: 14, justifyContent: 'center', borderRadius: 10, backgroundColor: value === name ? '#1A3D7C' : '#E5E7EB' }}>
          <Text style={{ color: value === name ? '#FFFFFF' : '#1C1C1E' }}>{name}</Text>
        </Pressable>
      ))}
    </ScrollView>
  );
}

function Screens() {
  const { signedIn } = useSession();
  const [screen, setScreen] = useState('Inbox');
  // A stack of what the person opened inside a screen: the kit raises events, the app navigates.
  const [stack, setStack] = useState([]);
  const push = (view) => setStack((s) => [...s, view]);
  const back = () => setStack((s) => s.slice(0, -1));
  const current = stack.at(-1);
  const refreshable = useRef(null);
  const [refreshing, setRefreshing] = useState(false);
  const refresh = async () => { setRefreshing(true); await refreshable.current?.refresh(); setRefreshing(false); };

  if (signedIn === null) return null;
  if (!signedIn) {
    return (
      <View style={{ padding: 16 }}>
        <SignIn bootstrap={{ authentication: { methods: { email: true } } }} redirectUri={REDIRECT_URI} openBrowser={openBrowser} allowPassword onSignedIn={() => {}} />
        <Tracking />
      </View>
    );
  }

  let body;
  if (current?.kind === 'order') body = <OrderDetail type={current.type} orderId={current.id} onTrack={(n) => push({ kind: 'track', n })} />;
  else if (current?.kind === 'track') body = <Tracking identifier={current.n} />;
  else if (current?.kind === 'store') body = <StorePage ref={refreshable} slug={current.slug} onOpenProduct={(slug, productId) => push({ kind: 'product', slug, productId })} onOpenCart={(slug) => push({ kind: 'cart', slug })} />;
  else if (current?.kind === 'product') body = <ProductDetail slug={current.slug} productId={current.productId} onAdded={back} onOpenGroupBuy={(code) => push({ kind: 'groupBuy', code })} />;
  else if (current?.kind === 'cart') body = <Cart slug={current.slug} onCheckout={(slug) => push({ kind: 'checkout', slug })} />;
  else if (current?.kind === 'checkout') {
    body = (
      <Checkout slug={current.slug} onPlaced={() => setStack([])}
        onPayment={(payment) => Alert.alert('Card payment', `Present your card sheet with ${payment.client_secret}.`)} />
    );
  } else if (current?.kind === 'groupBuy') {
    body = <GroupBuy code={current.code} onPayment={(payment) => Alert.alert('Card hold', `Present your card sheet with ${payment.client_secret}, then confirm the payment.`)} />;
  } else if (current?.kind === 'groupBuyOrder') {
    body = <GroupBuyOrder token={current.token} onPay={(token) => Alert.alert('Card hold', `Resume the card hold of order ${token}.`)} />;
  } else if (current?.kind === 'leadingGroup') {
    body = <GroupBuyLeadingGroup groupId={current.groupId} onShare={(share, url) => Share.share({ message: url, title: share?.title })} />;
  } else if (current?.kind === 'returnForm') body = <ReturnForm pickFile={pickFile} canCancel onCreated={back} onCancel={back} />;
  else if (current?.kind === 'stops') body = <RouteStops ref={refreshable} routeId={current.routeId} onOpenStop={(routeId, orderId) => push({ kind: 'stop', routeId, orderId })} />;
  else if (current?.kind === 'stop') {
    body = <Stop routeId={current.routeId} orderId={current.orderId} getPosition={getPosition} pickPhoto={pickPhoto} onCompleted={back}
      onCall={(phone) => Linking.openURL(`tel:${phone}`)} onNavigate={(lat, lng, address) => Linking.openURL(lat && lng ? `geo:${lat},${lng}` : `geo:0,0?q=${encodeURIComponent(address)}`)} />;
  } else if (screen === 'Inbox') body = <NotificationInbox ref={refreshable} onOpen={(n) => n.deep_link && Linking.openURL(n.deep_link)} />;
  else if (screen === 'Wallet') body = <WalletSummary ref={refreshable} onTopUp={({ minimum, maximum, currency }) => Alert.alert('Top up', `Between ${minimum} and ${maximum} ${currency}.`)} />;
  else if (screen === 'Orders') body = <OrderList ref={refreshable} onOpen={({ type, id }) => push({ kind: 'order', type, id })} />;
  else if (screen === 'Tracking') body = <Tracking />;
  else if (screen === 'Returns') body = <ReturnList ref={refreshable} canCreate onCreate={() => push({ kind: 'returnForm' })} />;
  else if (screen === 'Ship') body = <ShippingOrderForm onCreated={(order) => Alert.alert('Order placed', String(order.id))} />;
  else if (screen === 'Stores') body = <StoreDirectory ref={refreshable} onOpen={(slug) => push({ kind: 'store', slug })} />;
  else if (screen === 'Group buys') {
    body = (
      <View style={{ gap: 24 }}>
        <GroupBuyOrders ref={refreshable} onOpen={(token) => push({ kind: 'groupBuyOrder', token })} />
        <GroupBuyLeading onOpen={(groupId) => push({ kind: 'leadingGroup', groupId })} onOpenPlan={(code) => push({ kind: 'groupBuy', code })} />
      </View>
    );
  } else if (screen === 'Account') {
    body = (
      <View style={{ gap: 24 }}>
        <NotificationPreferences />
        <AccountPrivacy onOpenLink={(url) => Linking.openURL(url)} onClosureRequested={() => client.auth.signOut()} />
      </View>
    );
  } else if (screen === 'Driver') {
    body = (
      <View style={{ gap: 24 }}>
        <DriverDuty />
        <ScanLookup onFound={(order) => Alert.alert('Found', String(order.ref ?? order.id))} />
        <DriverRoutes ref={refreshable} onOpen={(routeId) => push({ kind: 'stops', routeId })} />
        <DriverEarnings />
      </View>
    );
  }

  return (
    <View style={{ flex: 1 }}>
      <Tabs value={screen} onChange={(name) => { setScreen(name); setStack([]); }} />
      {stack.length ? <Pressable onPress={back} accessibilityRole="button" style={{ minHeight: 44, justifyContent: 'center', paddingHorizontal: 16 }}><Text>‹ Back</Text></Pressable> : null}
      <ScrollView contentContainerStyle={{ padding: 16 }} keyboardShouldPersistTaps="handled" refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} />}>
        {body}
      </ScrollView>
    </View>
  );
}

export default function App() {
  // One client for the whole app; replacing it (another person or service provider) reloads every component.
  const value = useMemo(() => client, []);
  return (
    <SuperrouteProvider client={value}>
      <SafeAreaView style={{ flex: 1, backgroundColor: '#FFFFFF' }}>
        <Screens />
      </SafeAreaView>
    </SuperrouteProvider>
  );
}
