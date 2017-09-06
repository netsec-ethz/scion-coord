'use strict';

angular.module('scionApp', [
  'ngRoute', 'vcRecaptcha'

]).config(function ($routeProvider, $locationProvider, $httpProvider) {
    $routeProvider
      .when('/register', {
        templateUrl: '/public/partials/register.html',
        controller: 'registerCtrl',
        resolve: {
            ResolveSiteKey: ['registerService', function(registerService){
            return registerService.getSiteKey()
          }]
        }
      })
      .when('/login', {
        templateUrl: '/public/partials/login.html',
        controller: 'loginCtrl'
      })
      .when('/user', {
        templateUrl: '/public/partials/user.html',
        controller: 'userCtrl'
      })
      .otherwise({
        redirectTo: '/login'
      });

      $httpProvider.defaults.xsrfHeaderName = 'X-Xsrf-Token';

          $httpProvider.interceptors.push(function() {
              return {
                  response: function(response) {
                    console.log(response.headers('X-Xsrf-Token'));
                      $httpProvider.defaults.headers.common['X-Xsrf-Token'] = response.headers('X-Xsrf-Token');
                      return response;
                  }
              }
          });


    //$locationProvider.html5Mode(false);
  });
