'use strict';

const scionApp = angular.module('scionApp', [
    'ngRoute', 'vcRecaptcha'
]).config(function ($routeProvider, $locationProvider, $httpProvider) {
    $routeProvider
        .when('/register', {
            templateUrl: '/public/partials/register.html',
            controller: 'registerCtrl',
            resolve: {
                ResolveSiteKey: ['registerService', function (registerService) {
                    return registerService.getSiteKey()
                }]
            }
        })
        .when('/setPassword/:uuid', {
            templateUrl: '/public/partials/set_password.html',
            controller: 'passwordCtrl'
        })
        .when('/login', {
            templateUrl: '/public/partials/login.html',
            controller: 'loginCtrl'
        })
        .when('/user', {
            templateUrl: '/public/partials/user.html',
            controller: 'userCtrl'
        })
        .when('/resend', {
            templateUrl: '/public/partials/resend.html',
            controller: 'resendCtrl'
          })
        .when('/admin', {
            templateUrl: '/public/partials/admin.html',
            controller: 'adminCtrl'
        })
        .when('/verifyEmail/:uuid', {
            templateUrl: '/public/partials/email_verification.html',
            controller: 'verificationCtrl'
        })
        .otherwise({
            redirectTo: '/login'
        });

    $httpProvider.defaults.xsrfHeaderName = 'X-Xsrf-Token';

    $httpProvider.interceptors.push(function () {
        return {
            response: function (response) {
                console.log(response.headers('X-Xsrf-Token'));
                $httpProvider.defaults.headers.common['X-Xsrf-Token'] = response.headers('X-Xsrf-Token');
                return response;
            }
        }
    });
});

(function () {
    let directiveId = 'checkMatch';
    scionApp
        .directive(directiveId, ['$parse', function ($parse) {

            return {
                require: 'ngModel',
                link: function (scope, elem, attrs, ctrl) {
                    // if ngModel is not defined, we don't need to do anything
                    if (!ctrl) return;
                    if (!attrs[directiveId]) return;

                    let firstPassword = $parse(attrs[directiveId]);

                    let validator = function (value) {
                        let temp = firstPassword(scope),
                            v = value === temp;
                        ctrl.$setValidity('match', v);
                        return value;
                    };

                    ctrl.$parsers.unshift(validator);
                    ctrl.$formatters.push(validator);
                    attrs.$observe(directiveId, function () {
                        validator(ctrl.$viewValue);
                    });

                }
            };
        }]);
})();
