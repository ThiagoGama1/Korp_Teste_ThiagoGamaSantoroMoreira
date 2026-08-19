import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideHttpClient, withFetch } from '@angular/common/http';
import { provideRouter } from '@angular/router';

import { routes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes),

    // Registra o HttpClient no injetor. Sem isto, o inject(HttpClient) dos
    // services falha em tempo de execução — o Angular não injeta o que não foi
    // provido.
    //
    // withFetch troca o XMLHttpRequest antigo pela API fetch do navegador.
    provideHttpClient(withFetch()),
  ],
};
