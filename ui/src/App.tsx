import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import EasyModal from "ez-modal-react";
import { ToastContainer } from "react-toastify";
import { ThemeProvider } from "src/context";
import Router from "src/Router.tsx";

const queryClient = new QueryClient();

function App() {
	return (
		<ThemeProvider>
			<QueryClientProvider client={queryClient}>
				<EasyModal.Provider>
					<Router />
				</EasyModal.Provider>
				<ToastContainer
					position="top-right"
					autoClose={5000}
					hideProgressBar={true}
					newestOnTop={true}
					closeOnClick={true}
					rtl={false}
					closeButton={false}
				/>
				<ReactQueryDevtools buttonPosition="bottom-right" position="right" />
			</QueryClientProvider>
		</ThemeProvider>
	);
}

export default App;
