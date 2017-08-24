angular.module('scionApp')
    .factory('loginService', ["$http", "$q",'$httpParamSerializer', function($http, $q, $httpParamSerializer) {
    var loginService = {
        // Log the user in
        login: function(user) {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/login', user).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        logout: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/logout').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        resendEmail: function(email){
           return $http({
                method: 'POST',
                url: '/api/resendLink',
                data: $httpParamSerializer({ 'email': email }),
                headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' }
            });
        }
    };
    return loginService;
}]);
