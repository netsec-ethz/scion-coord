'use strict';

angular.module('scionApp', [
  'ngRoute'

]).config(function ($routeProvider, $locationProvider, $httpProvider) {
    $routeProvider
      .when('/register', {
        templateUrl: '/public/partials/register.html',
        controller: 'registerCtrl'
      })
      .when('/login', {
        templateUrl: '/public/partials/login.html',
        controller: 'loginCtrl'
      })
      .when('/admin', {
        templateUrl: '/public/partials/admin.html',
        controller: 'adminCtrl'
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
